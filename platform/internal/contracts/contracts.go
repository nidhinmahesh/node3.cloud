// Package contracts manages hosted smart contract deployment, the dapp callback
// endpoint that rubixgoplatform calls, WASM execution, and execution logging.
package contracts

import (
	"bytes"
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/tetratelabs/wazero"
	"github.com/tetratelabs/wazero/imports/wasi_snapshot_preview1"
	"github.com/tetratelabs/wazero/sys"
	"node3.cloud/platform/internal/auth"
	"node3.cloud/platform/internal/db"
	"node3.cloud/platform/internal/nodeutil"
)

// Handler owns contract management and the dapp callback endpoint.
type Handler struct {
	db             *db.DB
	wasmDir        string
	platformURL    string
	callbackSecret string // used to derive per-contract callback tokens
	client         *http.Client // for generate (60 s) and registerCallback
	deployClient   *http.Client // for deploy: consensus can take several minutes
}

func NewHandler(database *db.DB, wasmDir, platformURL, callbackSecret string) *Handler {
	os.MkdirAll(wasmDir, 0755)
	return &Handler{
		db:             database,
		wasmDir:        wasmDir,
		platformURL:    platformURL,
		callbackSecret: callbackSecret,
		client:         &http.Client{Timeout: 60 * time.Second},
		deployClient:   &http.Client{Timeout: 5 * time.Minute},
	}
}

// callbackToken derives a per-contract HMAC token from the shared secret.
// Logging the callback URL exposes this token, but it only authorises
// callbacks for the specific contractID — not the global secret.
func (h *Handler) callbackToken(contractID string) string {
	mac := hmac.New(sha256.New, []byte(h.callbackSecret))
	mac.Write([]byte(contractID))
	return hex.EncodeToString(mac.Sum(nil))
}

// SignatureNeededError is returned by deployOnNode when the node responds with
// "Signature needed" for a non-custodial DID — the caller must relay the
// sign_id and hash back to the browser so it can sign and call /api/tx/sign.
type SignatureNeededError struct {
	ID   string // reqID from the node
	Hash string // base64-encoded hash bytes
}

func (e *SignatureNeededError) Error() string {
	return "signature needed: " + e.ID
}

// ── HTTP handlers ─────────────────────────────────────────────────────────────

func (h *Handler) HandleList(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	cs, err := h.db.ListHostedContracts(r.Context(), account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]contractResponse, 0, len(cs))
	for _, c := range cs {
		out = append(out, contractToResponse(c))
	}
	writeJSON(w, http.StatusOK, out)
}

// HandleDeploy receives the .wasm binary + .rs source, stores them, then:
//  1. Calls rubixgoplatform to generate (upload to IPFS) the contract.
//  2. Registers our callback URL with the node.
//  3. Calls rubixgoplatform to deploy (consensus + genesis block).
func (h *Handler) HandleDeploy(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())

	if account.DID == "" {
		writeErr(w, http.StatusBadRequest, "create a DID before deploying a contract")
		return
	}

	// Free tier limit: 1 hosted contract. Fail closed on DB error.
	if account.Tier == "free" {
		cs, err := h.db.ListHostedContracts(r.Context(), account.ID)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "could not check contract limit")
			return
		}
		if len(cs) >= 1 {
			writeErr(w, http.StatusForbidden, "free tier limit: 1 hosted contract")
			return
		}
	}

	if err := r.ParseMultipartForm(32 << 20); err != nil {
		writeErr(w, http.StatusBadRequest, "multipart parse error")
		return
	}

	wasmBytes, err := readFormFile(r, "wasm")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "wasm file required")
		return
	}
	rsBytes, err := readFormFile(r, "source")
	if err != nil {
		writeErr(w, http.StatusBadRequest, "source file required")
		return
	}
	initialStateStr := r.FormValue("initial_state")
	if initialStateStr == "" {
		initialStateStr = "{}"
	}
	var initialState json.RawMessage
	if err := json.Unmarshal([]byte(initialStateStr), &initialState); err != nil {
		writeErr(w, http.StatusBadRequest, "initial_state must be valid JSON")
		return
	}

	// Generate the contract on the Rubix node (uploads WASM + RS to IPFS).
	contractID, err := h.generateOnNode(account.NodeID, account.DID, wasmBytes, rsBytes)
	if err != nil {
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("node generate: %v", err))
		return
	}

	// Persist the contract record (before deploy, so callback has it).
	dbContract := &db.HostedContract{
		AccountID:    account.ID,
		ContractID:   contractID,
		InitialState: initialState,
	}
	_, err = h.db.CreateHostedContract(r.Context(), dbContract)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, fmt.Sprintf("db: %v", err))
		return
	}

	// Store the WASM binary locally for execution.
	wasmPath := filepath.Join(h.wasmDir, contractID+".wasm")
	if err := os.WriteFile(wasmPath, wasmBytes, 0644); err != nil {
		// Roll back the DB row so the contract slot isn't permanently consumed.
		h.db.DeleteHostedContract(r.Context(), contractID)
		writeErr(w, http.StatusInternalServerError, "failed to store wasm")
		return
	}

	// Per-contract token: HMAC(secret, contractID). Logging the URL exposes
	// only this token, which is scoped to this contract — not the global secret.
	callbackURL := h.platformURL + "/internal/sc/callback?token=" + h.callbackToken(contractID)
	if err := h.registerCallback(account.NodeID, contractID, callbackURL); err != nil {
		log.Printf("register callback failed for %s: %v", contractID, err)
		// Non-fatal — manual re-registration possible.
	}

	// Deploy (consensus + genesis block). For custodial DIDs this returns immediately
	// on success. For non-custodial DIDs the node returns "Signature needed" — we
	// save the pending context and return HTTP 202 so the browser can sign and call
	// POST /api/tx/sign to complete consensus.
	deployErr := h.deployOnNode(account.NodeID, contractID, account.DID, initialStateStr)
	if deployErr != nil {
		var snErr *SignatureNeededError
		if errors.As(deployErr, &snErr) {
			// Save context so HandleTxSign can call MarkContractDeployed after signing.
			if err := h.db.SavePendingSignContext(r.Context(), snErr.ID, "deploy", contractID, account.ID); err != nil {
				log.Printf("node deploy %s: save sign context: %v", contractID, err)
			}
			writeJSON(w, http.StatusAccepted, map[string]interface{}{
				"needs_signature": true,
				"sign_id":         snErr.ID,
				"hash":            snErr.Hash,
				"contract_id":     contractID,
			})
			return
		}
		// Hard deploy failure: roll back DB + WASM so the slot is freed.
		h.db.DeleteHostedContract(r.Context(), contractID)   //nolint:errcheck
		os.Remove(wasmPath)                                   //nolint:errcheck
		writeErr(w, http.StatusBadGateway, fmt.Sprintf("node deploy: %v", deployErr))
		return
	}

	if err := h.db.MarkContractDeployed(r.Context(), contractID); err != nil {
		log.Printf("node deploy %s: mark deployed: %v", contractID, err)
	}

	if fetched, err := h.db.GetHostedContractByRubixID(r.Context(), contractID); err == nil {
		writeJSON(w, http.StatusOK, contractToResponse(*fetched))
	} else {
		writeJSON(w, http.StatusOK, contractToResponse(*dbContract))
	}
}

func (h *Handler) HandleExecutions(w http.ResponseWriter, r *http.Request) {
	account := auth.AccountFromCtx(r.Context())
	rowID, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeErr(w, http.StatusBadRequest, "invalid id")
		return
	}
	contract, err := h.db.GetHostedContractByRowID(r.Context(), rowID, account.ID)
	if err != nil {
		writeErr(w, http.StatusNotFound, "contract not found")
		return
	}
	contractID := contract.ContractID

	execs, err := h.db.ListContractExecutions(r.Context(), contractID, account.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	out := make([]executionResponse, 0, len(execs))
	for _, e := range execs {
		out = append(out, execToResponse(e))
	}
	writeJSON(w, http.StatusOK, out)
}

// HandleSCCallback is called by rubixgoplatform when a smart contract executes.
// The node sends: {smart_contract_hash, port, smart_contract_data, initiator_did}
// We must respond HTTP 200 with {message: "..."}.
func (h *Handler) HandleSCCallback(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SmartContractHash string `json:"smart_contract_hash"`
		Port              int    `json:"port"`
		SmartContractData string `json:"smart_contract_data"` // JSON string
		InitiatorDID      string `json:"initiator_did"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeErr(w, http.StatusBadRequest, "invalid payload")
		return
	}

	contractID := req.SmartContractHash

	// Verify the per-contract HMAC token. We parse the body first so we have
	// the contractID to derive the expected token — the token is scoped to
	// this contract, so logging the URL does not expose the global secret.
	if r.URL.Query().Get("token") != h.callbackToken(contractID) {
		writeErr(w, http.StatusUnauthorized, "invalid callback token")
		return
	}
	contract, err := h.db.GetHostedContractByRubixID(context.Background(), contractID)
	if err != nil {
		log.Printf("SC callback: unknown contract %s: %v", contractID, err)
		writeErr(w, http.StatusNotFound, "contract not found")
		return
	}

	wasmPath := filepath.Join(h.wasmDir, contractID+".wasm")
	wasmBytes, err := os.ReadFile(wasmPath)
	if err != nil {
		log.Printf("SC callback: missing wasm for %s", contractID)
		writeErr(w, http.StatusInternalServerError, "wasm not found")
		return
	}

	// Validate and parse the input data from the node.
	var inputData json.RawMessage
	if req.SmartContractData != "" {
		if !json.Valid([]byte(req.SmartContractData)) {
			writeErr(w, http.StatusBadRequest, "smart_contract_data is not valid JSON")
			return
		}
		inputData = json.RawMessage(req.SmartContractData)
	} else {
		inputData = json.RawMessage(`{}`)
	}

	// Use a bounded context: WASM must complete within 30 seconds.
	wasmCtx, wasmCancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer wasmCancel()

	stateBefore := contract.CurrentState
	newState, output, execErr := runWASM(wasmCtx, wasmBytes, inputData, stateBefore)

	success := execErr == nil
	errMsg := ""
	if execErr != nil {
		errMsg = execErr.Error()
		newState = stateBefore // rollback to previous state on error
		output = json.RawMessage(`{}`)
		log.Printf("SC callback: wasm error for %s: %v", contractID, execErr)
	}

	// Persist state and execution log.
	if err := h.db.UpdateContractState(context.Background(), contractID, newState); err != nil {
		log.Printf("SC callback: update state failed: %v", err)
	}
	h.db.InsertContractExecution(context.Background(), &db.ContractExecution{
		ContractID:   contractID,
		InitiatorDID: req.InitiatorDID,
		Input:        inputData,
		Output:       output,
		StateBefore:  stateBefore,
		StateAfter:   newState,
		Success:      success,
		Error:        errMsg,
	})

	// Node expects HTTP 200 with a JSON message field.
	msg := "executed"
	if !success {
		msg = "execution failed: " + errMsg
	}
	writeJSON(w, http.StatusOK, map[string]string{"message": msg})
}

// ── WASM execution ────────────────────────────────────────────────────────────

// runWASM executes a contract WASM module.
//
// Contract interface (WASI-based):
//   - stdin:  JSON object {"input": <inputData>, "state": <currentState>}
//   - stdout: JSON object {"state": <newState>, "output": <returnValue>}
//   - Non-zero exit = error; stderr is the error message.
func runWASM(ctx context.Context, wasmBytes []byte, input, state json.RawMessage) (newState json.RawMessage, output json.RawMessage, err error) {
	rt := wazero.NewRuntimeWithConfig(ctx,
		wazero.NewRuntimeConfigInterpreter().WithCloseOnContextDone(true))
	defer rt.Close(ctx)

	wasi_snapshot_preview1.MustInstantiate(ctx, rt)

	stdin := map[string]json.RawMessage{"input": input, "state": state}
	stdinBytes, _ := json.Marshal(stdin)

	var stdoutBuf, stderrBuf bytes.Buffer

	cfg := wazero.NewModuleConfig().
		WithStdin(bytes.NewReader(stdinBytes)).
		WithStdout(&stdoutBuf).
		WithStderr(&stderrBuf).
		WithArgs("contract")

	mod, compileErr := rt.CompileModule(ctx, wasmBytes)
	if compileErr != nil {
		return nil, nil, fmt.Errorf("compile: %w", compileErr)
	}

	_, runErr := rt.InstantiateModule(ctx, mod, cfg)
	if runErr != nil {
		// WASI programs call proc_exit(0) on clean exit; wazero returns a non-nil
		// *sys.ExitError even for exit code 0. Treat code 0 as success and fall
		// through to parse stdout. Any non-zero exit code is a contract failure.
		var exitErr *sys.ExitError
		if !errors.As(runErr, &exitErr) || exitErr.ExitCode() != 0 {
			errDetail := strings.TrimSpace(stderrBuf.String())
			if errDetail == "" {
				errDetail = runErr.Error()
			}
			return nil, nil, fmt.Errorf("run: %s", errDetail)
		}
	}

	var result struct {
		State  json.RawMessage `json:"state"`
		Output json.RawMessage `json:"output"`
	}
	if err := json.Unmarshal(stdoutBuf.Bytes(), &result); err != nil {
		return nil, nil, fmt.Errorf("invalid wasm output: %w", err)
	}
	if result.State == nil {
		result.State = state // no state change
	}
	if result.Output == nil {
		result.Output = json.RawMessage(`null`)
	}
	return result.State, result.Output, nil
}

// ── Node communication ────────────────────────────────────────────────────────

// generateOnNode uploads WASM + RS source to the rubixgoplatform node via
// multipart/form-data (the format the node's APIGenerateSmartContract handler
// expects). Field names and file extensions must match exactly.
func (h *Handler) generateOnNode(nodeIndex int, did string, wasmBytes, rsBytes []byte) (string, error) {
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)

	// text field: did
	if err := mw.WriteField("did", did); err != nil {
		return "", err
	}
	// file field: binaryCodePath (.wasm extension required by node validation)
	wasmPart, err := mw.CreateFormFile("binaryCodePath", "contract.wasm")
	if err != nil {
		return "", err
	}
	if _, err := wasmPart.Write(wasmBytes); err != nil {
		return "", err
	}
	// file field: rawCodePath (.rs extension required by node validation)
	rsPart, err := mw.CreateFormFile("rawCodePath", "contract.rs")
	if err != nil {
		return "", err
	}
	if _, err := rsPart.Write(rsBytes); err != nil {
		return "", err
	}
	mw.Close()

	url := nodeutil.URL(nodeIndex) + "/rubix/v1/smart_contracts/generate"
	resp, err := h.client.Post(url, mw.FormDataContentType(), &buf)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	// The node responds with models.BasicResponse: {status, message, result}
	// where result is the smart contract token hash string.
	var result struct {
		Status  bool        `json:"status"`
		Message string      `json:"message"`
		Result  interface{} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if !result.Status {
		return "", fmt.Errorf("node error: %s", result.Message)
	}
	contractID, _ := result.Result.(string)
	if contractID == "" {
		return "", fmt.Errorf("node returned empty contract hash")
	}
	return contractID, nil
}

func (h *Handler) registerCallback(nodeIndex int, contractID, callbackURL string) error {
	// Field names match models.RegisterCallBackUrlReq in rubixgoplatform.
	body, _ := json.Marshal(map[string]string{
		"SmartContractToken": contractID,
		"CallBackURL":        callbackURL,
	})
	regClient := &http.Client{Timeout: 15 * time.Second}
	url := nodeutil.URL(nodeIndex) + "/rubix/v1/smart_contracts/register_callback"
	resp, err := regClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 300 {
		return fmt.Errorf("node returned %d", resp.StatusCode)
	}
	return nil
}

func (h *Handler) deployOnNode(nodeIndex int, contractID, deployerDID, initialState string) error {
	// POST /rubix/v1/tx expects a TransactionRequest (models.TransactionRequest).
	// Smart contract deploy is submitted as a SmartContractInfo entry under Tokens.
	body, _ := json.Marshal(map[string]interface{}{
		"initiator": deployerDID,
		"owner":     deployerDID,
		"tokens": map[string]interface{}{
			"smartContract": []map[string]interface{}{
				{
					"smartContractId": contractID,
					"value":           0,
					"data":            initialState,
				},
			},
		},
	})
	url := nodeutil.URL(nodeIndex) + "/rubix/v1/tx"
	resp, err := h.deployClient.Post(url, "application/json", bytes.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Non-custodial DIDs have no private key on the node; the node sends back
	// "Signature needed" which the browser must handle via the signing relay.
	var result struct {
		Status  bool   `json:"status"`
		Message string `json:"message"`
		Result  struct {
			ID   string `json:"id"`
			Hash string `json:"hash"` // base64-encoded bytes in JSON ([]byte marshals to base64)
		} `json:"result"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode deploy response: %w", err)
	}
	if result.Message == "Signature needed" {
		return &SignatureNeededError{ID: result.Result.ID, Hash: result.Result.Hash}
	}
	if !result.Status {
		return fmt.Errorf("node deploy error: %s", result.Message)
	}
	return nil
}

// ── Helpers ──────────────────────────────────────────────────────────────────

func readFormFile(r *http.Request, field string) ([]byte, error) {
	f, _, err := r.FormFile(field)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return io.ReadAll(io.LimitReader(f, 10<<20)) // 10 MB cap
}

// ── Response types ────────────────────────────────────────────────────────────

type contractResponse struct {
	ID             string          `json:"id"`
	ContractID     string          `json:"contract_id"`
	DeployedAt     *string         `json:"deployed_at"`
	ExecutionCount int             `json:"execution_count"`
	CurrentState   json.RawMessage `json:"current_state"`
}

type executionResponse struct {
	ID           string          `json:"id"`
	ExecutedAt   string          `json:"executed_at"`
	InitiatorDID string          `json:"initiator_did"`
	Input        json.RawMessage `json:"input"`
	Output       json.RawMessage `json:"output"`
	StateBefore  json.RawMessage `json:"state_before"`
	StateAfter   json.RawMessage `json:"state_after"`
	Success      bool            `json:"success"`
	Error        *string         `json:"error"`
}

func contractToResponse(c db.HostedContract) contractResponse {
	r := contractResponse{
		ID:             strconv.FormatInt(c.ID, 10),
		ContractID:     c.ContractID,
		ExecutionCount: c.ExecutionCount,
		CurrentState:   c.CurrentState,
	}
	if c.DeployedAt != nil {
		s := c.DeployedAt.Format(time.RFC3339)
		r.DeployedAt = &s
	}
	if r.CurrentState == nil {
		r.CurrentState = json.RawMessage(`{}`)
	}
	return r
}

func execToResponse(e db.ContractExecution) executionResponse {
	r := executionResponse{
		ID:           strconv.FormatInt(e.ID, 10),
		ExecutedAt:   e.ExecutedAt.Format(time.RFC3339),
		InitiatorDID: e.InitiatorDID,
		Input:        e.Input,
		Output:       e.Output,
		StateBefore:  e.StateBefore,
		StateAfter:   e.StateAfter,
		Success:      e.Success,
	}
	if e.Error != "" {
		r.Error = &e.Error
	}
	if r.Input == nil {
		r.Input = json.RawMessage(`null`)
	}
	if r.Output == nil {
		r.Output = json.RawMessage(`null`)
	}
	return r
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

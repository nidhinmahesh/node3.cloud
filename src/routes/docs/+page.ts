import { redirect } from '@sveltejs/kit';

// Redirect to the rubixgoplatform API reference until node3.cloud has hosted docs.
export const load = () => {
	redirect(302, 'https://github.com/rubixchain/rubixgoplatform/blob/release-v1/README.md');
};

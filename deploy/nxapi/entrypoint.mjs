// Starts nxapi's HTTP server for the KFIRE Nintendo connector.
//
// nxapi-auth issues no plain client secret yet, only a shared secret that
// signs client assertions: this mints one (HS256, valid one year, renewed at
// every start) and hands it to nxapi. Without credentials the container stays
// up doing nothing, so a stack without the Nintendo connector keeps working.
import { createHmac } from 'node:crypto';
import { spawn } from 'node:child_process';

const clientId = process.env.KFIRE_NXAPI_CLIENT_ID ?? '';
const secret = process.env.KFIRE_NXAPI_SHARED_SECRET ?? '';

if (!clientId || !secret) {
	console.log('kfire-nxapi: KFIRE_NXAPI_CLIENT_ID / KFIRE_NXAPI_SHARED_SECRET not set, Nintendo connector off');
	setInterval(() => {}, 1 << 30);
} else {
	const b64 = (o) => Buffer.from(JSON.stringify(o)).toString('base64url');
	const now = Math.floor(Date.now() / 1000);
	const claims = {
		iss: clientId, sub: clientId, aud: 'https://nxapi-auth.fancy.org.uk',
		typ: 'client_assertion', iat: now, exp: now + 365 * 86400,
	};
	const signing = b64({ alg: 'HS256', typ: 'JWT' }) + '.' + b64(claims);
	const assertion = signing + '.' + createHmac('sha256', secret).update(signing).digest('base64url');

	const child = spawn('nxapi', ['--data-path', '/data', 'nso', 'http-server', '--listen', '0.0.0.0:8080'], {
		stdio: 'inherit',
		env: {
			...process.env,
			NXAPI_ZNCA_API_CLIENT_ID: clientId,
			NXAPI_ZNCA_API_CLIENT_ASSERTION: assertion,
			KFIRE_NXAPI_SHARED_SECRET: '',
		},
	});
	child.on('exit', (code) => process.exit(code ?? 1));
	for (const sig of ['SIGTERM', 'SIGINT']) process.on(sig, () => child.kill(sig));
}

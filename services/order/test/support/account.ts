import { Server, createServer } from 'node:http';
import { AddressInfo } from 'node:net';

/** Answers /api/v1/auth/me and /api/v1/users/me like auth and user do. */
export class AccountServer {
  phone: string | null = '+2250700000001';
  displayName: string | null = 'Awa Koné';
  authStatus = 200;
  userStatus = 200;
  delayMs = 0;
  readonly authorizations: string[] = [];
  private server: Server | undefined;

  async start(): Promise<string> {
    this.server = createServer((req, res) => {
      this.authorizations.push(req.headers.authorization ?? '');
      const auth = req.url === '/api/v1/auth/me';
      const status = auth ? this.authStatus : this.userStatus;
      const data = auth
        ? { id: 'x', phone: this.phone, roles: ['PERSONAL'] }
        : { user_id: 'x', display_name: this.displayName };
      setTimeout(() => {
        res.statusCode = status;
        res.setHeader('content-type', 'application/json');
        res.end(JSON.stringify({ data, error: null, meta: { request_id: 'r' } }));
      }, this.delayMs);
    });
    await new Promise<void>((resolve) => this.server?.listen(0, '127.0.0.1', resolve));
    const { port } = this.server.address() as AddressInfo;
    return `http://127.0.0.1:${port}`;
  }

  async stop(): Promise<void> {
    this.server?.closeAllConnections();
    await new Promise((resolve) => this.server?.close(resolve));
  }
}

import { execFileSync } from 'node:child_process';
import path from 'node:path';

import { PostgreSqlContainer, StartedPostgreSqlContainer } from '@testcontainers/postgresql';

export interface TestDatabase {
  container: StartedPostgreSqlContainer;
  url: string;
}

/** Starts Postgres and applies the real migrations with `prisma migrate deploy`. */
export async function startDatabase(): Promise<TestDatabase> {
  const container = await new PostgreSqlContainer('postgres:16-alpine')
    .withDatabase('poro_notification')
    .withUsername('poro')
    .withPassword('poro_test_password')
    .start();
  const url = `postgresql://poro:poro_test_password@127.0.0.1:${container.getPort()}/poro_notification?sslmode=disable`;
  const root = path.resolve(__dirname, '../..');
  execFileSync(
    process.execPath,
    [path.join(root, 'node_modules/prisma/build/index.js'), 'migrate', 'deploy'],
    {
      cwd: root,
      env: { ...process.env, DATABASE_URL: url },
      stdio: 'pipe',
    },
  );
  return { container, url };
}

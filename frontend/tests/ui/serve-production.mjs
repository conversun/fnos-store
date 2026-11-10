import { createServer } from 'node:http';
import { readFile } from 'node:fs/promises';
import { fileURLToPath } from 'node:url';
import { extname, resolve, sep } from 'node:path';

const webRoot = fileURLToPath(new URL('../../../web/', import.meta.url));
const gatewayPrefix = '/app/fnos-apps-store';
const mimeTypes = {
  '.html': 'text/html',
  '.js': 'text/javascript',
  '.css': 'text/css',
  '.svg': 'image/svg+xml',
};

// Serve the actual build at root and at the fnOS gateway mount. Tests intercept APIs.
createServer(async (request, response) => {
  const url = new URL(request.url || '/', 'http://127.0.0.1');
  const path = url.pathname.startsWith(`${gatewayPrefix}/`)
    ? url.pathname.slice(gatewayPrefix.length)
    : url.pathname;
  const file = resolve(webRoot, `.${path === '/' ? '/index.html' : path}`);
  if (!file.startsWith(`${resolve(webRoot)}${sep}`) || path.startsWith('/api/')) {
    response.writeHead(404).end();
    return;
  }
  try {
    const bytes = await readFile(file);
    response.writeHead(200, { 'Content-Type': mimeTypes[extname(file)] || 'application/octet-stream' });
    response.end(bytes);
  } catch (error) {
    if (error instanceof Error && 'code' in error && error.code === 'ENOENT') {
      response.writeHead(404).end();
      return;
    }
    response.writeHead(500).end();
    console.error(error);
  }
}).listen(Number(process.env.UI_TEST_PORT || 4191), '127.0.0.1');

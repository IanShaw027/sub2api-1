#!/usr/bin/env node

const http = require('node:http');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const assert = require('node:assert/strict');
const { Transform, pipeline } = require('node:stream');
const { randomUUID } = require('node:crypto');
const { mkdirSync, openSync, writeSync, closeSync, chmodSync, statSync, mkdtempSync, rmSync } = require('node:fs');

const DEFAULT_LISTEN = '127.0.0.1:8787';
const DEFAULT_TARGET = 'https://crs.qazwc.com';
const DEFAULT_LOG_DIR = path.join(os.tmpdir(), 'crs-capture-logs');
const DEFAULT_MAX_BODY_BYTES = 8 * 1024 * 1024;
const DEFAULT_UPSTREAM_TIMEOUT_MS = 10 * 60 * 1000;

function printUsage() {
  process.stdout.write(
    [
      'Usage: node tools/crs_capture_proxy.js [options]',
      '',
      'Options:',
      '  --listen <host:port>           Listen address (default: 127.0.0.1:8787)',
      '  --target <url>                 Upstream target base URL (default: https://crs.qazwc.com)',
      '  --log-dir <path>               JSONL output directory (default: /tmp/crs-capture-logs)',
      '  --max-body-bytes <n>           Max bytes captured per request/response body (default: 8388608)',
      '  --upstream-timeout-ms <n>      Upstream timeout in milliseconds (default: 600000)',
      '  --self-check                   Run local invariants and exit',
      '  --help                         Show this help message',
      '',
      'Example:',
      '  node tools/crs_capture_proxy.js --listen 127.0.0.1:3001',
      '  # then point base_url at http://127.0.0.1:3001',
      '',
    ].join('\n'),
  );
}

function parseArgs(argv) {
  const args = {
    listen: DEFAULT_LISTEN,
    target: DEFAULT_TARGET,
    logDir: DEFAULT_LOG_DIR,
    maxBodyBytes: DEFAULT_MAX_BODY_BYTES,
    upstreamTimeoutMs: DEFAULT_UPSTREAM_TIMEOUT_MS,
    selfCheck: false,
    help: false,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const value = argv[i];
    if (value === '--help' || value === '-h') {
      args.help = true;
      continue;
    }
    if (value === '--self-check') {
      args.selfCheck = true;
      continue;
    }
    if (!value.startsWith('--')) {
      throw new Error(`unexpected argument: ${value}`);
    }
    const key = value.slice(2);
    const next = argv[i + 1];
    if (next == null || next.startsWith('--')) {
      throw new Error(`missing value for --${key}`);
    }
    i += 1;
    switch (key) {
      case 'listen':
        args.listen = next;
        break;
      case 'target':
        args.target = next;
        break;
      case 'log-dir':
        args.logDir = next;
        break;
      case 'max-body-bytes':
        args.maxBodyBytes = parsePositiveInt(next, 'max-body-bytes');
        break;
      case 'upstream-timeout-ms':
        args.upstreamTimeoutMs = parsePositiveInt(next, 'upstream-timeout-ms');
        break;
      default:
        throw new Error(`unknown option: --${key}`);
    }
  }

  return args;
}

function parsePositiveInt(value, name) {
  return parseStrictNonNegativeInt(value, name);
}

function parseHostPort(value) {
  const trimmed = String(value || '').trim();
  if (!trimmed) {
    throw new Error('listen address is empty');
  }
  if (trimmed.startsWith('[')) {
    const end = trimmed.indexOf(']');
    if (end < 0) {
      throw new Error(`invalid listen address: ${value}`);
    }
    const host = trimmed.slice(1, end);
    const remainder = trimmed.slice(end + 1);
    if (!remainder.startsWith(':')) {
      throw new Error(`invalid listen address: ${value}`);
    }
    const port = parsePort(remainder.slice(1), value);
    return { host, port };
  }
  const parts = trimmed.split(':');
  if (parts.length < 2) {
    throw new Error(`invalid listen address: ${value}`);
  }
  const port = parsePort(parts.pop(), value);
  const host = parts.join(':') || '127.0.0.1';
  return { host, port };
}

function parsePort(value, original) {
  const trimmed = String(value).trim();
  if (!/^\d+$/.test(trimmed)) {
    throw new Error(`invalid listen port in ${original}`);
  }
  const parsed = Number(trimmed);
  if (!Number.isSafeInteger(parsed) || parsed > 65535) {
    throw new Error(`invalid listen port in ${original}`);
  }
  return parsed;
}

function parseStrictNonNegativeInt(value, name) {
  const trimmed = String(value).trim();
  if (!/^\d+$/.test(trimmed)) {
    throw new Error(`invalid ${name}: ${value}`);
  }
  const parsed = Number(trimmed);
  if (!Number.isSafeInteger(parsed)) {
    throw new Error(`invalid ${name}: ${value}`);
  }
  return parsed;
}

function buildUpstreamUrl(targetBase, incomingUrl) {
  const upstreamBase = new URL(targetBase);
  const incoming = splitRequestTarget(incomingUrl);
  const basePath = upstreamBase.pathname === '/' ? '' : upstreamBase.pathname.replace(/\/+$/, '');
  return {
    protocol: upstreamBase.protocol,
    hostname: upstreamBase.hostname,
    port: upstreamBase.port || undefined,
    host: upstreamBase.host,
    origin: upstreamBase.origin,
    path: `${basePath}${incoming.path}${incoming.query}`,
    targetPath: `${basePath}${incoming.path}`,
  };
}

function cloneHeaders(headers) {
  const out = {};
  for (const [key, value] of Object.entries(headers || {})) {
    out[key] = value;
  }
  return out;
}

function splitRequestTarget(rawUrl) {
  const raw = rawUrl == null || rawUrl === '' ? '/' : String(rawUrl);
  const queryIndex = raw.indexOf('?');
  const beforeQuery = queryIndex >= 0 ? raw.slice(0, queryIndex) : raw;
  const query = queryIndex >= 0 ? raw.slice(queryIndex) : '';
  const schemeMatch = beforeQuery.match(/^[a-zA-Z][a-zA-Z0-9+.-]*:\/\//);
  if (!schemeMatch) {
    return {
      path: beforeQuery || '/',
      query,
    };
  }
  const authorityStart = schemeMatch[0].length;
  const pathStart = beforeQuery.indexOf('/', authorityStart);
  return {
    path: pathStart >= 0 ? beforeQuery.slice(pathStart) : '/',
    query,
  };
}

function getConnectionHeaderTokens(headers) {
  const connectionValue = headers?.connection;
  if (!connectionValue) {
    return [];
  }
  return String(connectionValue)
    .split(',')
    .map((token) => token.trim().toLowerCase())
    .filter(Boolean);
}

function stripHopByHopHeaders(headers) {
  const out = cloneHeaders(headers);
  for (const key of getConnectionHeaderTokens(out)) {
    delete out[key];
  }
  for (const key of [
    'connection',
    'proxy-connection',
    'keep-alive',
    'proxy-authenticate',
    'proxy-authorization',
    'transfer-encoding',
    'te',
    'trailer',
    'upgrade',
  ]) {
    delete out[key];
  }
  return out;
}

function sanitizeOutgoingHeaders(req, targetUrl) {
  const headers = stripHopByHopHeaders(req.headers);
  headers.host = targetUrl.host;
  headers['x-forwarded-host'] = req.headers.host || '';
  headers['x-forwarded-proto'] = req.socket.encrypted ? 'https' : 'http';
  const forwardedFor = buildForwardedFor(req);
  if (forwardedFor) {
    headers['x-forwarded-for'] = forwardedFor;
  } else {
    delete headers['x-forwarded-for'];
  }
  headers['accept-encoding'] = 'identity';
  return headers;
}

function sanitizeIncomingResponseHeaders(headers) {
  return stripHopByHopHeaders(headers);
}

function sanitizeHeadersForLog(headers) {
  const out = stripHopByHopHeaders(headers);
  for (const [key] of Object.entries(out)) {
    switch (key.toLowerCase()) {
      case 'authorization':
      case 'proxy-authorization':
      case 'cookie':
      case 'set-cookie':
      case 'x-api-key':
      case 'x-api-token':
      case 'x-access-token':
      case 'x-auth-token':
      case 'x-client-secret':
      case 'x-csrf-token':
      case 'x-xsrf-token':
      case 'x-goog-api-key':
      case 'x-rapidapi-key':
      case 'x-amz-security-token':
      case 'api-key':
      case 'api-token':
      case 'x-token':
      case 'token':
      case 'referer':
      case 'location':
        out[key] = '[redacted]';
        break;
      default:
        break;
    }
  }
  return out;
}

function sanitizeQueryForLog(query) {
  if (!query) {
    return '';
  }
  const raw = query.startsWith('?') ? query.slice(1) : query;
  if (!raw) {
    return '';
  }
  const params = new URLSearchParams(raw);
  const parts = [];
  for (const [key, value] of params) {
    const sanitizedValue = shouldRedactQueryParam(key.toLowerCase()) ? '[redacted]' : value;
    parts.push(`${encodeURIComponent(key)}=${encodeURIComponent(sanitizedValue)}`);
  }
  return parts.length > 0 ? `?${parts.join('&')}` : '';
}

function shouldRedactQueryParam(name) {
  return [
    'authorization',
    'auth',
    'api_key',
    'apikey',
    'access_token',
    'refresh_token',
    'id_token',
    'code_verifier',
    'client_assertion',
    'assertion',
    'jwt',
    'token',
    'secret',
    'client_secret',
    'password',
    'passwd',
    'code',
    'session',
    'session_id',
    'sid',
  ].includes(name);
}

function sanitizeBodyTextForLog(headers, bodyText) {
  if (!isTextualContentType(headers?.['content-type'])) {
    return '[omitted non-text body]';
  }
  if (!bodyText) {
    return '';
  }
  return redactSecretLikeText(bodyText);
}

function isTextualContentType(contentType) {
  const value = String(contentType || '').toLowerCase();
  if (!value) {
    return true;
  }
  return (
    value.startsWith('text/') ||
    value.includes('json') ||
    value.includes('xml') ||
    value.includes('x-www-form-urlencoded')
  );
}

function redactSecretLikeText(text) {
  let redacted = String(text);
  redacted = redacted.replace(/(Bearer\s+)[A-Za-z0-9._~+/=-]+/gi, '$1[redacted]');
  redacted = redacted.replace(/(Basic\s+)[A-Za-z0-9+/=]+/gi, '$1[redacted]');
  redacted = redacted.replace(
    /(["'])(password|passwd|api[_-]?key|access[_-]?token|refresh[_-]?token|id[_-]?token|code[_-]?verifier|client[_-]?assertion|assertion|jwt|client[_-]?secret|secret|token|authorization)\1(\s*:\s*)(["'])([^"\\]*(?:\\.[^"\\]*)*)\4/gi,
    (_, keyQuote, keyName, separator, valueQuote) => `${keyQuote}${keyName}${keyQuote}${separator}${valueQuote}[redacted]${valueQuote}`,
  );
  redacted = redacted.replace(
    /((?:password|passwd|api[_-]?key|access[_-]?token|refresh[_-]?token|id[_-]?token|code[_-]?verifier|client[_-]?assertion|assertion|jwt|client[_-]?secret|secret|token|authorization)\s*[:=]\s*)(["']?)([^"'\s,&}\]]+)(\2?)/gi,
    '$1[redacted]',
  );
  return redacted;
}

function buildForwardedFor(req) {
  const remote = req.socket.remoteAddress || '';
  return remote;
}

function formatLoggedUpstreamUrl(upstreamUrl, incomingUrl) {
  return `${upstreamUrl.origin}${upstreamUrl.targetPath}${sanitizeQueryForLog(incomingUrl.query)}`;
}

class BodyRecorder extends Transform {
  constructor(limitBytes) {
    super();
    this.limitBytes = limitBytes;
    this.totalBytes = 0;
    this.capturedBytes = 0;
    this.parts = [];
    this.truncated = false;
  }

  _transform(chunk, encoding, callback) {
    const buf = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk, encoding);
    this.totalBytes += buf.length;
    if (this.capturedBytes < this.limitBytes) {
      const remaining = this.limitBytes - this.capturedBytes;
      const slice = buf.subarray(0, Math.min(remaining, buf.length));
      if (slice.length > 0) {
        this.parts.push(Buffer.from(slice));
        this.capturedBytes += slice.length;
      }
      if (buf.length > remaining) {
        this.truncated = true;
      }
    } else {
      this.truncated = true;
    }
    this.push(buf);
    callback();
  }

  snapshot() {
    const body = this.parts.length > 0 ? Buffer.concat(this.parts, this.capturedBytes) : Buffer.alloc(0);
    return {
      bodyBytes: this.totalBytes,
      bodyCapturedBytes: this.capturedBytes,
      bodyTruncated: this.truncated,
      bodyText: body.toString('utf8'),
    };
  }
}

function ensureLogDir(logDir) {
  mkdirSync(logDir, { recursive: true });
  chmodSync(logDir, 0o700);
}

function logPathFor(logDir, now = new Date()) {
  const day = now.toISOString().slice(0, 10);
  return path.join(logDir, `crs-capture-${day}.jsonl`);
}

function appendJsonl(logDir, record) {
  const line = `${JSON.stringify(record)}\n`;
  const logPath = logPathFor(logDir);
  const fd = openSync(logPath, 'a', 0o600);
  try {
    writeSync(fd, line, undefined, 'utf8');
    chmodSync(logDir, 0o700);
    chmodSync(logPath, 0o600);
  } finally {
    closeSync(fd);
  }
}

function getIncomingUrl(req) {
  return splitRequestTarget(req.url || '/');
}

function createProxyServer(config) {
  const server = http.createServer((req, res) => {
    void handleRequest(req, res, config);
  });

  server.on('checkContinue', (req, res) => {
    res.writeContinue();
    void handleRequest(req, res, config);
  });

  return server;
}

async function handleRequest(req, res, config) {
  const requestStartedAt = Date.now();
  const requestId = randomUUID();
  const incomingUrl = getIncomingUrl(req);

  if (incomingUrl.path === '/__capture/healthz') {
    res.writeHead(200, { 'content-type': 'application/json; charset=utf-8' });
    res.end(
      JSON.stringify({
        ok: true,
        request_id: requestId,
        target: config.target,
        log_dir: config.logDir,
      }),
    );
    return;
  }

  const upstreamUrl = buildUpstreamUrl(config.target, req.url || '/');
  const outgoingHeaders = sanitizeOutgoingHeaders(req, upstreamUrl);
  const requestRecorder = new BodyRecorder(config.maxBodyBytes);
  const responseState = {
    recorder: null,
    statusCode: null,
    headers: null,
    statusMessage: null,
  };

  const abortController = new AbortController();
  let finalized = false;
  let upstreamReq = null;

  const finalize = (patch = {}) => {
    if (finalized) {
      return;
    }
    finalized = true;
    const requestSnapshot = requestRecorder.snapshot();
    const responseSnapshot = responseState.recorder ? responseState.recorder.snapshot() : {
      bodyBytes: 0,
      bodyCapturedBytes: 0,
      bodyTruncated: false,
      bodyText: '',
    };
    appendJsonl(config.logDir, {
      id: requestId,
      ts: new Date(requestStartedAt).toISOString(),
      elapsed_ms: Date.now() - requestStartedAt,
      request: {
        method: req.method,
        path: incomingUrl.path,
        query: sanitizeQueryForLog(incomingUrl.query),
        headers: sanitizeHeadersForLog(req.headers),
        remote_address: req.socket.remoteAddress,
        remote_port: req.socket.remotePort,
        body_bytes: requestSnapshot.bodyBytes,
        body_captured_bytes: requestSnapshot.bodyCapturedBytes,
        body_truncated: requestSnapshot.bodyTruncated,
        body_text: sanitizeBodyTextForLog(req.headers, requestSnapshot.bodyText),
      },
      upstream: {
        url: formatLoggedUpstreamUrl(upstreamUrl, incomingUrl),
        method: req.method,
        headers: sanitizeHeadersForLog(outgoingHeaders),
      },
      response: {
        status_code: responseState.statusCode,
        status_message: responseState.statusMessage,
        headers: sanitizeHeadersForLog(responseState.headers),
        body_bytes: responseSnapshot.bodyBytes,
        body_captured_bytes: responseSnapshot.bodyCapturedBytes,
        body_truncated: responseSnapshot.bodyTruncated,
        body_text: sanitizeBodyTextForLog(responseState.headers, responseSnapshot.bodyText),
      },
      error: patch.error || null,
      aborted: Boolean(patch.aborted),
      client_closed: Boolean(patch.client_closed),
      upstream_closed: Boolean(patch.upstream_closed),
    });

    const statusLabel = responseState.statusCode == null ? 'ERR' : String(responseState.statusCode);
    const reqSize = formatBytes(requestSnapshot.bodyBytes);
    const resSize = formatBytes(responseSnapshot.bodyBytes);
    const suffix = patch.error ? ` error=${patch.error}` : '';
    process.stdout.write(
      `[${requestId}] ${req.method} ${incomingUrl.path} -> ${statusLabel} ` +
        `req=${reqSize} res=${resSize}${suffix}\n`,
    );
  };

  const client = upstreamUrl.protocol === 'http:' ? http : https;
  const upstreamOptions = {
    protocol: upstreamUrl.protocol,
    hostname: upstreamUrl.hostname,
    port: upstreamUrl.port || undefined,
    method: req.method,
    path: upstreamUrl.path,
    headers: outgoingHeaders,
    signal: abortController.signal,
  };

  req.on('aborted', () => {
    abortController.abort();
    finalize({ aborted: true, error: 'client aborted request' });
  });

  try {
    upstreamReq = client.request(upstreamOptions, (upstreamRes) => {
      responseState.statusCode = upstreamRes.statusCode || 502;
      responseState.statusMessage = upstreamRes.statusMessage || '';
      responseState.headers = sanitizeIncomingResponseHeaders(upstreamRes.headers);
      responseState.recorder = new BodyRecorder(config.maxBodyBytes);

      res.writeHead(responseState.statusCode, responseState.headers);

      const onPipelineDone = (err) => {
        if (err && !finalized) {
          finalize({ error: err.message });
          return;
        }
        finalize();
      };

      pipeline(upstreamRes, responseState.recorder, res, onPipelineDone);

      upstreamRes.on('aborted', () => {
        if (!finalized) {
          finalize({ upstream_closed: true, error: 'upstream aborted response' });
        }
      });
    });

    upstreamReq.setTimeout(config.upstreamTimeoutMs, () => {
      upstreamReq.destroy(new Error(`upstream timeout after ${config.upstreamTimeoutMs}ms`));
    });

    upstreamReq.on('error', (err) => {
      if (res.headersSent || res.writableEnded) {
        if (!finalized) {
          finalize({ error: err.message });
        }
        return;
      }
      const message = `upstream request failed: ${err.message}`;
      res.writeHead(502, { 'content-type': 'application/json; charset=utf-8' });
      res.end(JSON.stringify({ error: message }));
      finalize({ error: message });
    });

    pipeline(req, requestRecorder, upstreamReq, (err) => {
      if (err && !finalized) {
        finalize({ error: err.message });
      }
    });
  } catch (err) {
    const message = err instanceof Error ? err.message : String(err);
    if (!res.headersSent) {
      res.writeHead(500, { 'content-type': 'application/json; charset=utf-8' });
      res.end(JSON.stringify({ error: message }));
    }
    finalize({ error: message });
  }
}

function formatBytes(bytes) {
  if (!Number.isFinite(bytes) || bytes < 0) {
    return '0B';
  }
  const units = ['B', 'KB', 'MB', 'GB'];
  let value = bytes;
  let unitIndex = 0;
  while (value >= 1024 && unitIndex < units.length - 1) {
    value /= 1024;
    unitIndex += 1;
  }
  const rounded = value >= 10 || unitIndex === 0 ? Math.round(value) : Math.round(value * 10) / 10;
  return `${rounded}${units[unitIndex]}`;
}

async function main() {
  const args = parseArgs(process.argv.slice(2));
  if (args.help) {
    printUsage();
    return;
  }
  if (args.selfCheck) {
    runSelfCheck();
    return;
  }

  const listen = parseHostPort(args.listen);
  ensureLogDir(args.logDir);

  const server = createProxyServer({
    target: args.target,
    logDir: args.logDir,
    maxBodyBytes: args.maxBodyBytes,
    upstreamTimeoutMs: args.upstreamTimeoutMs,
  });

  server.listen(listen.port, listen.host, () => {
    const address = server.address();
    const actualPort = address && typeof address === 'object' ? address.port : listen.port;
    process.stdout.write(
      `crs capture proxy listening on http://${listen.host}:${actualPort} -> ${args.target}\n` +
        `logs: ${logPathFor(args.logDir)}\n` +
        `health: http://${listen.host}:${actualPort}/__capture/healthz\n`,
    );
  });

  const shutdown = (signal) => {
    process.stdout.write(`received ${signal}, shutting down\n`);
    server.close(() => process.exit(0));
    setTimeout(() => process.exit(1), 5000).unref();
  };

  process.on('SIGINT', () => shutdown('SIGINT'));
  process.on('SIGTERM', () => shutdown('SIGTERM'));
}

function runSelfCheck() {
  assert.equal(
    buildUpstreamUrl('https://example.test/base', '/a/./b/%2e%2e/c?x=1').path,
    '/base/a/./b/%2e%2e/c?x=1',
  );
  assert.equal(
    formatLoggedUpstreamUrl(
      buildUpstreamUrl('https://example.test/base', '/a/./b/%2e%2e/c?token=abc123&keep=ok'),
      { query: '?token=abc123&keep=ok' },
    ),
    'https://example.test/base/a/./b/%2e%2e/c?token=%5Bredacted%5D&keep=ok',
  );
  assert.equal(
    sanitizeQueryForLog('?token=abc123&keep=ok'),
    '?token=%5Bredacted%5D&keep=ok',
  );
  assert.equal(
    sanitizeHeadersForLog({ authorization: 'Bearer abc123', 'content-type': 'text/plain' }).authorization,
    '[redacted]',
  );
  assert.equal(
    sanitizeHeadersForLog({
      referer: 'https://example.test/oauth/start?code=abc123',
      location: 'https://example.test/oauth/callback?jwt=abc123',
    }).referer,
    '[redacted]',
  );
  assert.equal(
    sanitizeHeadersForLog({
      referer: 'https://example.test/oauth/start?code=abc123',
      location: 'https://example.test/oauth/callback?jwt=abc123',
    }).location,
    '[redacted]',
  );
  assert.equal(
    sanitizeHeadersForLog({ 'x-api-key': 'secret', 'content-type': 'text/plain' })['x-api-key'],
    '[redacted]',
  );
  assert.equal(
    sanitizeQueryForLog('?code_verifier=abc123&client_assertion=def456&assertion=ghi789&jwt=jkl012&keep=ok'),
    '?code_verifier=%5Bredacted%5D&client_assertion=%5Bredacted%5D&assertion=%5Bredacted%5D&jwt=%5Bredacted%5D&keep=ok',
  );
  assert.equal(
    sanitizeBodyTextForLog(
      { 'content-type': 'application/json' },
      '{"api_key":"abc123","code_verifier":"pkce","client_assertion":"jwt","assertion":"signed","jwt":"token","nested":{"client_secret":"shh"}}',
    ),
    '{"api_key":"[redacted]","code_verifier":"[redacted]","client_assertion":"[redacted]","assertion":"[redacted]","jwt":"[redacted]","nested":{"client_secret":"[redacted]"}}',
  );
  assert.equal(
    sanitizeBodyTextForLog(
      { 'content-type': 'multipart/form-data; boundary=abc' },
      '------abc\r\nContent-Disposition: form-data; name="api_key"\r\n\r\nabc123\r\n------abc--',
    ),
    '[omitted non-text body]',
  );
  assert.deepEqual(
    stripHopByHopHeaders({
      connection: 'X-Foo, Keep-Alive',
      'x-foo': '1',
      'proxy-authorization': 'secret',
      'keep-alive': 'timeout=5',
      'content-type': 'text/plain',
    }),
    {
      'content-type': 'text/plain',
    },
  );
  assert.equal(
    sanitizeOutgoingHeaders(
      {
        headers: {
          host: 'client.example',
          'x-forwarded-for': '1.2.3.4, 5.6.7.8',
        },
        socket: { encrypted: false, remoteAddress: '9.9.9.9' },
      },
      { host: 'upstream.example' },
    )['x-forwarded-for'],
    '9.9.9.9',
  );
  assert.throws(() => parsePositiveInt('12abc', 'max-body-bytes'));
  assert.throws(() => parsePositiveInt('1.5', 'upstream-timeout-ms'));
  assert.throws(() => parsePort('80x', '127.0.0.1:80x'));

  const tempDir = mkdtempSync(path.join(os.tmpdir(), 'crs-capture-self-check-'));
  try {
    ensureLogDir(tempDir);
    appendJsonl(tempDir, { ok: true });
    const logFile = logPathFor(tempDir);
    assert.equal(statSync(tempDir).mode & 0o777, 0o700);
    assert.equal(statSync(logFile).mode & 0o777, 0o600);
  } finally {
    rmSync(tempDir, { recursive: true, force: true });
  }

  process.stdout.write('self-check passed\n');
}

main().catch((err) => {
  const message = err instanceof Error ? err.message : String(err);
  process.stderr.write(`${message}\n`);
  process.exit(1);
});

#!/usr/bin/env node

const http = require('node:http');
const https = require('node:https');
const os = require('node:os');
const path = require('node:path');
const { Transform, pipeline } = require('node:stream');
const { randomUUID } = require('node:crypto');
const { mkdirSync, appendFileSync } = require('node:fs');

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
    help: false,
  };

  for (let i = 0; i < argv.length; i += 1) {
    const value = argv[i];
    if (value === '--help' || value === '-h') {
      args.help = true;
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
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed < 0) {
    throw new Error(`invalid ${name}: ${value}`);
  }
  return parsed;
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
  const parsed = Number.parseInt(value, 10);
  if (!Number.isFinite(parsed) || parsed < 0 || parsed > 65535) {
    throw new Error(`invalid listen port in ${original}`);
  }
  return parsed;
}

function buildUpstreamUrl(targetBase, incomingUrl) {
  const upstreamBase = new URL(targetBase);
  const incoming = new URL(incomingUrl, 'http://placeholder.local');
  const basePath = upstreamBase.pathname === '/' ? '' : upstreamBase.pathname.replace(/\/+$/, '');
  upstreamBase.pathname = `${basePath}${incoming.pathname}`;
  upstreamBase.search = incoming.search;
  upstreamBase.hash = '';
  return upstreamBase;
}

function cloneHeaders(headers) {
  const out = {};
  for (const [key, value] of Object.entries(headers || {})) {
    out[key] = value;
  }
  return out;
}

function stripHopByHopHeaders(headers) {
  const out = cloneHeaders(headers);
  for (const key of [
    'connection',
    'proxy-connection',
    'keep-alive',
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
  headers['x-forwarded-for'] = buildForwardedFor(req);
  headers['accept-encoding'] = 'identity';
  return headers;
}

function sanitizeIncomingResponseHeaders(headers) {
  return stripHopByHopHeaders(headers);
}

function buildForwardedFor(req) {
  const prior = String(req.headers['x-forwarded-for'] || '').trim();
  const remote = req.socket.remoteAddress || '';
  if (!prior) {
    return remote;
  }
  if (!remote) {
    return prior;
  }
  return `${prior}, ${remote}`;
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
}

function logPathFor(logDir, now = new Date()) {
  const day = now.toISOString().slice(0, 10);
  return path.join(logDir, `crs-capture-${day}.jsonl`);
}

function appendJsonl(logDir, record) {
  const line = `${JSON.stringify(record)}\n`;
  appendFileSync(logPathFor(logDir), line, 'utf8');
}

function summarizeHeaders(headers) {
  return Object.fromEntries(Object.entries(headers || {}));
}

function getIncomingUrl(req) {
  const rawUrl = req.url || '/';
  try {
    return new URL(rawUrl);
  } catch {
    return new URL(rawUrl, `http://${req.headers.host || 'localhost'}`);
  }
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

  if (incomingUrl.pathname === '/__capture/healthz') {
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
        url: req.url,
        path: incomingUrl.pathname,
        query: incomingUrl.search,
        headers: summarizeHeaders(req.headers),
        remote_address: req.socket.remoteAddress,
        remote_port: req.socket.remotePort,
        body_bytes: requestSnapshot.bodyBytes,
        body_captured_bytes: requestSnapshot.bodyCapturedBytes,
        body_truncated: requestSnapshot.bodyTruncated,
        body_text: requestSnapshot.bodyText,
      },
      upstream: {
        url: upstreamUrl.toString(),
        method: req.method,
        headers: summarizeHeaders(outgoingHeaders),
      },
      response: {
        status_code: responseState.statusCode,
        status_message: responseState.statusMessage,
        headers: responseState.headers,
        body_bytes: responseSnapshot.bodyBytes,
        body_captured_bytes: responseSnapshot.bodyCapturedBytes,
        body_truncated: responseSnapshot.bodyTruncated,
        body_text: responseSnapshot.bodyText,
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
      `[${requestId}] ${req.method} ${req.url} -> ${statusLabel} ` +
        `req=${reqSize} res=${resSize}${suffix}\n`,
    );
  };

  const client = upstreamUrl.protocol === 'http:' ? http : https;
  const upstreamOptions = {
    protocol: upstreamUrl.protocol,
    hostname: upstreamUrl.hostname,
    port: upstreamUrl.port || undefined,
    method: req.method,
    path: `${upstreamUrl.pathname}${upstreamUrl.search}`,
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

main().catch((err) => {
  const message = err instanceof Error ? err.message : String(err);
  process.stderr.write(`${message}\n`);
  process.exit(1);
});

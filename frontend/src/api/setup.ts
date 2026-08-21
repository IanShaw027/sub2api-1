/**
 * Setup API endpoints
 */
import axios from 'axios'
import { buildGatewayUrl } from './url'

const setupBootstrapSecretStorageKey = 'setup_bootstrap_secret'
const setupBootstrapSecretHeader = 'X-Setup-Bootstrap-Secret'

export function configureSetupBootstrapSecretFromLocation(location: Location = window.location): void {
  const prefix = '#setup-secret='
  if (!location.hash.startsWith(prefix)) return

  sessionStorage.removeItem(setupBootstrapSecretStorageKey)
  try {
    const secret = decodeURIComponent(location.hash.slice(prefix.length)).trim()
    if (secret) {
      sessionStorage.setItem(setupBootstrapSecretStorageKey, secret)
    }
  } catch {
    // Malformed fragments fail closed instead of reusing an older secret.
  } finally {
    history.replaceState(history.state, '', `${location.pathname}${location.search}`)
  }
}

export function clearSetupBootstrapSecret(): void {
  sessionStorage.removeItem(setupBootstrapSecretStorageKey)
}

// Create a separate client for setup endpoints (not under /api/v1)
const setupClient = axios.create({
  baseURL: buildGatewayUrl('/').replace(/\/+$/, ''),
  timeout: 30000,
  headers: {
    'Content-Type': 'application/json'
  }
})

setupClient.interceptors.request.use((config) => {
  const secret = sessionStorage.getItem(setupBootstrapSecretStorageKey)
  if (secret) {
    config.headers.set(setupBootstrapSecretHeader, secret)
  }
  return config
})

export interface SetupStatus {
  needs_setup: boolean
  step: string
}

export interface DatabaseConfig {
  host: string
  port: number
  user: string
  password: string
  dbname: string
  sslmode: string
}

export interface RedisConfig {
  host: string
  port: number
  username: string
  password: string
  db: number
  enable_tls: boolean
}

export interface AdminConfig {
  email: string
  password: string
}

export interface ServerConfig {
  host: string
  port: number
  mode: string
}

export interface InstallRequest {
  database: DatabaseConfig
  redis: RedisConfig
  admin: AdminConfig
  server: ServerConfig
}

export interface InstallResponse {
  message: string
  restart: boolean
}

/**
 * Get setup status
 */
export async function getSetupStatus(): Promise<SetupStatus> {
  const response = await setupClient.get('/setup/status')
  return response.data.data
}

/**
 * Test database connection
 */
export async function testDatabase(config: DatabaseConfig): Promise<void> {
  await setupClient.post('/setup/test-db', config)
}

/**
 * Test Redis connection
 */
export async function testRedis(config: RedisConfig): Promise<void> {
  await setupClient.post('/setup/test-redis', config)
}

/**
 * Perform installation
 */
export async function install(config: InstallRequest): Promise<InstallResponse> {
  const response = await setupClient.post('/setup/install', config)
  clearSetupBootstrapSecret()
  return response.data.data
}

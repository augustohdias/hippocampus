import * as fs from 'fs';

const SERVICE_NAME = "hippocampus-plugin";
const LOG_FILE = "/tmp/hippocampus-plugin.log";

/**
 * Logs debug messages to file when HIPPOCAMPUS_DEBUG=true.
 * Uses file logging instead of stdout/stderr to avoid breaking OpenCode protocol.
 */
export function log(_message: string, _data?: Record<string, unknown>): void {
  if (process.env.HIPPOCAMPUS_DEBUG === "true") {
    try {
      const timestamp = new Date().toISOString();
      const dataStr = _data ? ` ${JSON.stringify(_data, null, 2)}` : "";
      const logLine = `[${timestamp}] [${SERVICE_NAME}] ${_message}${dataStr}\n`;
      
      // Write to file synchronously to ensure logs aren't lost
      fs.appendFileSync(LOG_FILE, logLine, { encoding: 'utf8' });
    } catch (error) {
      // If file writing fails, fallback to silent (no output)
      // We can't use console.error as it would break OpenCode
    }
  }
}

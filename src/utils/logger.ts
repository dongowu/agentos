import pino from "pino";

const level = process.env.LOG_LEVEL ?? "info";

const transport =
  process.env.NODE_ENV !== "production"
    ? { target: "pino-pretty", options: { colorize: true } }
    : undefined;

const logger = pino({ level, transport });

export function createLogger(name: string): pino.Logger {
  return logger.child({ name });
}

export default logger;

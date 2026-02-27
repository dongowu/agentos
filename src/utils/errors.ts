export class BaseError extends Error {
  constructor(
    message: string,
    public readonly code: string,
    public readonly cause?: Error,
  ) {
    super(message);
    this.name = this.constructor.name;
  }
}

export class LLMError extends BaseError {
  constructor(
    message: string,
    public readonly provider: string,
    public readonly model: string,
    cause?: Error,
  ) {
    super(message, "LLM_ERROR", cause);
  }
}

export class ToolError extends BaseError {
  constructor(
    message: string,
    public readonly toolName: string,
    cause?: Error,
  ) {
    super(message, "TOOL_ERROR", cause);
  }
}

export class PipelineError extends BaseError {
  constructor(
    message: string,
    public readonly pipelineId: string,
    public readonly stage: string,
    cause?: Error,
  ) {
    super(message, "PIPELINE_ERROR", cause);
  }
}

export class ConfigError extends BaseError {
  constructor(message: string, cause?: Error) {
    super(message, "CONFIG_ERROR", cause);
  }
}

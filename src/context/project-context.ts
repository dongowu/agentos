import { generateId } from "../utils/id.js";
import type { ArtifactRepository } from "../storage/repository.js";
import type { Message } from "../messaging/types.js";

export interface Artifact {
  id: string;
  pipelineId: string;
  stageType: string;
  artifactType: string;
  name: string;
  filePath: string;
  contentHash?: string;
  version: number;
  createdBy: string;
  createdAt: string;
}

export interface StageContext {
  stageId: string;
  artifacts: Artifact[];
  messages: Message[];
}

export interface FullContext {
  pipelineId: string;
  artifacts: Artifact[];
  messages: Message[];
}

export class ProjectContext {
  private localArtifacts: Artifact[] = [];
  private localMessages: Message[] = [];

  constructor(
    private pipelineId: string,
    private artifactRepo?: ArtifactRepository,
  ) {}

  async getFullContext(): Promise<FullContext> {
    const repoArtifacts = this.artifactRepo
      ? this.toArtifacts(this.artifactRepo.findByPipelineId(this.pipelineId))
      : [];
    return {
      pipelineId: this.pipelineId,
      artifacts: [...repoArtifacts, ...this.localArtifacts],
      messages: [...this.localMessages],
    };
  }

  async getStageContext(stageId: string): Promise<StageContext> {
    const full = await this.getFullContext();
    return {
      stageId,
      artifacts: full.artifacts.filter((a) => a.stageType === stageId),
      messages: full.messages.filter((m) => m.subject === stageId),
    };
  }

  addArtifact(artifact: Omit<Artifact, "id" | "createdAt">): Artifact {
    const full: Artifact = {
      ...artifact,
      id: generateId("art"),
      createdAt: new Date().toISOString(),
    };
    this.localArtifacts.push(full);
    return full;
  }

  getArtifacts(stageType?: string, artifactType?: string): Artifact[] {
    let result = [...this.localArtifacts];
    if (stageType) {
      result = result.filter((a) => a.stageType === stageType);
    }
    if (artifactType) {
      result = result.filter((a) => a.artifactType === artifactType);
    }
    return result;
  }

  private toArtifacts(rows: ReturnType<ArtifactRepository["findByPipelineId"]>): Artifact[] {
    return rows.map((r) => ({
      id: r.id,
      pipelineId: r.pipeline_id,
      stageType: r.stage_type,
      artifactType: r.artifact_type,
      name: r.name,
      filePath: r.file_path,
      contentHash: r.content_hash ?? undefined,
      version: r.version,
      createdBy: r.created_by,
      createdAt: r.created_at,
    }));
  }
}

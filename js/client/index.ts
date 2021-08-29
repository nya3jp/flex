export interface Job {
  id: string
  spec: JobSpec
}

export interface JobSpec {
  command: JobCommand
  inputs: JobInputs
  limits: JobLimits
  constraints: JobConstraints
  annotations: JobAnnotations
}

export interface JobCommand {
  args: string[]
}

export interface JobInputs {
  packages: JobPackage[]
}

export interface JobPackage {
  hash: string
  tag: string
  installDir: string
}

export interface JobLimits {
  time: string
}

export interface JobConstraints {
  priority: number
}

export interface JobAnnotations {
  labels: string[]
}

export type JobState = 'UNSPECIFIED' | 'PENDING' | 'RUNNING' | 'FINISHED';

export interface JobStatus {
  job: Job
  state: JobState
  taskId: string
  flexletName: string
  result: TaskResult
}

export interface TaskResult {
  exitCode: number
  message: string
  time: string | null
}

export interface FlexletStatus {
  flexlet: Flexlet
  state: FlexletState
  currentJobs: Job[]
}

export interface Flexlet {
  name: string
  spec: FlexletSpec
}

export interface FlexletSpec {
  cores: number
}

export type FlexletState = 'OFFLINE' | 'ONLINE';

export interface Stats {
  job: JobStats
  flexlet: FlexletStats
}

export interface JobStats {
  pendingJobs: number
  runningJobs: number
}

export interface FlexletStats {
  onlineFlexlets: number
  offlineFlexlets: number
  busyCores: number
  idleCores: number
}

export interface ListJobsParams {
  limit?: number
  before?: string
  state?: JobState
  label?: string
}

export type JobOutputType = 'stdout' | 'stderr';

interface ListJobsResponse {
  jobs: JobStatus[]
}

interface GetJobResponse {
  job: JobStatus
}

interface ListFlexletsResponse {
  flexlets: FlexletStatus[]
}

interface GetStatsResponse {
  stats: Stats
}

export class FlexClient {
  constructor(private readonly baseUrl: string) {
  }

  public async listJobs(params?: ListJobsParams): Promise<JobStatus[]> {
    const search = new URLSearchParams();
    const limit = params?.limit;
    if (limit !== undefined) {
      search.set('limit', limit.toString());
    }
    const before = params?.before;
    if (before !== undefined) {
      search.set('before', before);
    }
    const state = params?.state;
    if (state !== undefined) {
      search.set('state', state);
    }
    const label = params?.label;
    if (label !== undefined) {
      search.set('label', label);
    }

    const res = await this.fetchJson<ListJobsResponse>('api/jobs?' + search.toString());
    return res.jobs;
  }

  public async getJob(id: string): Promise<JobStatus> {
    const res = await this.fetchJson<GetJobResponse>(`api/jobs/${id}`);
    return res.job;
  }

  public getJobOutput(id: string, type: JobOutputType): Promise<Response> {
    return this.fetch(`api/jobs/${id}/${type}`);
  }

  public async listFlexlets(): Promise<FlexletStatus[]> {
    const res = await this.fetchJson<ListFlexletsResponse>(`api/flexlets`);
    return res.flexlets;
  }

  public async getStats(): Promise<Stats> {
    const res = await this.fetchJson<GetStatsResponse>('api/stats');
    return res.stats;
  }

  private async fetchJson<T>(relUrl: string, init?: RequestInit): Promise<T> {
    const res = await this.fetch(relUrl, init);
    return (await res.json()) as T;
  }

  private fetch(relUrl: string, init?: RequestInit): Promise<Response> {
    const url = new URL(relUrl, this.baseUrl).toString();
    return fetch(url, init);
  }
}

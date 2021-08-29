import {JobStatus} from 'flex-client';

export function stateLabel(job: JobStatus): React.ReactElement {
  switch (job.state) {
    case 'PENDING':
      return <span className="badge bg-secondary">Pending</span>;
    case 'RUNNING':
      return <span className="badge bg-warning">Running</span>;
    case 'FINISHED':
      if (job.result.exitCode !== 0) {
        return <span className="badge bg-danger">Failure</span>;
      }
      return <span className="badge bg-success">Success</span>;
    default:
      return <span className="badge bg-dark">{job.state}</span>;
  }
};

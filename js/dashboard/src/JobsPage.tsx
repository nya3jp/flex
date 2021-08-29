import {useFlexClient} from './client';
import {JobStatus, ListJobsParams} from 'flex-client';
import {useEffect, useState} from 'react';
import {Link, useLocation} from 'react-router-dom';
import shellEscape from 'shell-escape';
import {stateLabel} from './commonui';
import './JobsPage.css';
import './text.css';

interface QueryParams {
  limit: number
  before: string | null
}

function useQueryParams(): QueryParams {
  const query = new URLSearchParams(useLocation().search);
  const limit = parseInt(query.get('limit') ?? '100');
  const before = query.get('before');
  return {limit, before};
}

function TableRow({job}: { job: JobStatus }): React.ReactElement {
  return (
      <tr>
        <td>
          <Link to={`/jobs/${job.job.id}`} className="text-reset">
            {job.job.id}
          </Link>
        </td>
        <td>
          {stateLabel(job)}
        </td>
        <td className="command">
          <Link to={`/jobs/${job.job.id}`} className="text-reset">
            {shellEscape(job.job.spec.command.args)}
          </Link>
        </td>
      </tr>
  );
}

function Table({jobs}: { jobs: JobStatus[] }): React.ReactElement {
  const {limit} = useQueryParams();
  return (
      <div>
        <table className="table table-sm table-striped jobs"
               style={{tableLayout: 'fixed'}}>
          <colgroup>
            <col style={{width: '6rem'}}/>
            <col style={{width: '6rem'}}/>
            <col/>
          </colgroup>
          <thead>
          <tr>
            <th scope="col">ID</th>
            <th scope="col">Status</th>
            <th scope="col">Command</th>
          </tr>
          </thead>
          <tbody>
          {jobs.map((job) => <TableRow key={job.job.id} job={job}/>)}
          </tbody>
        </table>
        <p style={{textAlign: 'right'}}>
          {
            jobs.length > 0 ?
                <Link
                    to={`/jobs/?limit=${limit}&before=${jobs[jobs.length - 1].job.id}`}>Older &raquo;</Link> :
                null
          }
        </p>
      </div>
  );
}

export default function JobsPage() {
  const client = useFlexClient();
  const {limit, before} = useQueryParams();
  const [jobs, setJobs] = useState<JobStatus[] | undefined>(undefined);
  useEffect(() => {
    (async () => {
      const params: ListJobsParams = {limit};
      if (before) {
        params.before = before;
      }
      const jobs = await client.listJobs(params);
      setJobs(jobs);
    })();
  }, [client, setJobs, limit, before]);

  if (jobs === undefined) {
    return <div>Loading...</div>;
  }

  return (
      <div>
        <h1>Jobs</h1>
        <Table jobs={jobs}/>
      </div>
  );
}

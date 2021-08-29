import {useFlexClient} from './client';
import {FlexClient, JobOutputType, JobStatus} from 'flex-client';
import {Fragment, useEffect, useState} from 'react';
import {Link, useParams} from 'react-router-dom';
import shellEscape from 'shell-escape';
import {stateLabel} from './commonui';
import './JobPage.css';
import './text.css';

interface JobPageParams {
  id: string
}

interface JobInfo {
  job: JobStatus
  stdout?: string
  stderr?: string
}

async function safeGetJobOutput(client: FlexClient, id: string, type: JobOutputType): Promise<string | undefined> {
  try {
    const res = await client.getJobOutput(id, type);
    if (!res.ok) {
      throw new Error(res.statusText);
    }
    return res.text();
  } catch (e) {
    return undefined;
  }
}

function JobInfoTable({job}: { job: JobStatus }): React.ReactElement {
  const packageItems = job.job.spec.inputs.packages.map((p) => (
      <li>
        {p.hash}
        {
          p.tag !== '' ?
              <span>(<Link to={`/packages/${p.tag}/`}>{p.tag}</Link>)</span> :
              null
        }
      </li>
  ));

  const labelItems = job.job.spec.annotations.labels.map((label) => (
      <code>{label}</code>
  ));

  let finishedRows = null;
  if (job.state === 'FINISHED') {
    finishedRows = (
        <Fragment>
          <tr>
            <th scope="row">Result</th>
            <td>
              {job.result.message}
            </td>
          </tr>
          <tr>
            <th scope="row">Run time</th>
            <td>
              {job.result.time}
            </td>
          </tr>
        </Fragment>
    );
  }

  return (
      <table className="table table-sm" style={{tableLayout: 'fixed'}}>
        <colgroup>
          <col style={{width: '6rem'}}/>
          <col/>
        </colgroup>
        <tbody>
        <tr>
          <th scope="row">ID</th>
          <td>{job.job.id}</td>
        </tr>
        <tr>
          <th scope="row">Command</th>
          <td className="command">{shellEscape(job.job.spec.command.args)}</td>
        </tr>
        <tr>
          <th scope="row">Packages</th>
          <td>
            {
              packageItems.length > 0 ?
                  <ul className="list-unstyled mb-0">
                    {packageItems}
                  </ul> :
                  <span>(No package)</span>
            }
          </td>
        </tr>
        <tr>
          <th scope="row">Labels</th>
          <td>
            {
              labelItems.length > 0 ?
                  <span>{labelItems}</span> :
                  <span>(No label)</span>
            }
          </td>
        </tr>
        {finishedRows}
        </tbody>
      </table>
  );
}

function JobOutput({
                     title,
                     job,
                     out
                   }: { title: string, job: JobStatus, out?: string }): React.ReactElement {
  if (job.state !== 'FINISHED') {
    return <Fragment/>;
  }
  return (
      <Fragment>
        <h2 className="mt-3">{title}</h2>
        {
          out !== undefined ?
              <div className="card">
                <div className="card-body wrapped-code">{out}</div>
              </div> :
              <div className="alert alert-danger" role="alert">ERROR: Failed to
                load</div>
        }
      </Fragment>
  );
}

export default function JobPage() {
  const client = useFlexClient();
  const {id} = useParams<JobPageParams>();
  const [jobInfo, setJobInfo] = useState<JobInfo | undefined>(undefined);
  useEffect(() => {
    (async () => {
      const job = await client.getJob(id);
      const stdout = job.state === 'FINISHED' ? (await safeGetJobOutput(client, id, 'stdout')) : undefined;
      const stderr = job.state === 'FINISHED' ? (await safeGetJobOutput(client, id, 'stderr')) : undefined;
      setJobInfo({job, stdout, stderr});
    })();
  }, [client, id, setJobInfo]);

  if (jobInfo === undefined) {
    return <div>Loading...</div>;
  }

  const {job, stdout, stderr} = jobInfo;

  return (
      <div>
        <h1>
          Job {job.job.id}
          &nbsp;
          <span style={{fontSize: '1rem'}}>
            {stateLabel(job)}
          </span>
        </h1>

        <JobInfoTable job={job}/>
        <JobOutput title="Standard Output" job={job} out={stdout}/>
        <JobOutput title="Standard Error" job={job} out={stderr}/>
      </div>
  );
}

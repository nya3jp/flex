import {useFlexClient} from './client';
import {FlexletStatus, Job} from 'flex-client';
import {useEffect, useState} from 'react';
import {Link} from 'react-router-dom';
import shellEscape from 'shell-escape';
import './FlexletsPage.css';
import './text.css';

function JobItem({job}: { job: Job }): React.ReactElement {
  return (
      <li>
        <Link to={`/jobs/${job.id}/`} className="text-reset">
          [{job.id}]
          <code className="command">{shellEscape(job.spec.command.args)}</code>
        </Link>
      </li>
  );
}

function TableRow({flexlet}: { flexlet: FlexletStatus }): React.ReactElement {
  const badge =
      flexlet.state === 'ONLINE' ?
          <span className="badge bg-success">Online</span> :
          <span className="badge bg-secondary">Offline</span>;
  return (
      <tr>
        <td>{badge}</td>
        <td className="text-ellipsis">{flexlet.flexlet.name}</td>
        <td>
          {flexlet.currentJobs.length} / {flexlet.flexlet.spec.cores}
        </td>
        <td>
          <ul className="list-unstyled mb-0">
            {flexlet.currentJobs.map((job) => <JobItem key={job.id}
                                                       job={job}/>)}
          </ul>
        </td>
      </tr>
  );
}

function Table({flexlets}: { flexlets: FlexletStatus[] }): React.ReactElement {
  return (
      <div>
        <table className="table table-sm table-striped flexlets"
               style={{tableLayout: 'fixed'}}>
          <colgroup>
            <col style={{width: '6rem'}}/>
            <col style={{width: '12rem'}}/>
            <col style={{width: '4rem'}}/>
            <col/>
          </colgroup>
          <thead>
          <tr>
            <th scope="col">State</th>
            <th scope="col">Name</th>
            <th scope="col">Cores</th>
            <th scope="col">Jobs</th>
          </tr>
          </thead>
          <tbody>
          {flexlets.map((flexlet) => <TableRow key={flexlet.flexlet.name}
                                               flexlet={flexlet}/>)}
          </tbody>
        </table>
      </div>
  );
}

export default function FlexletsPage() {
  const client = useFlexClient();
  const [flexlets, setFlexlets] = useState<FlexletStatus[] | undefined>(undefined);
  useEffect(() => {
    (async () => {
      const flexlets = await client.listFlexlets();
      setFlexlets(flexlets);
    })();
  }, [client, setFlexlets]);

  if (flexlets === undefined) {
    return <div>Loading...</div>;
  }

  return (
      <div>
        <h1>Flexlets</h1>
        <Table flexlets={flexlets}/>
      </div>
  );
}

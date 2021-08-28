import {useFlexClient} from './client';
import {Stats} from 'flex-client';
import {useEffect, useState} from 'react';

function StatsCardGroup({stats}: { stats: Stats }) {
  return (
      <div className="card-group">
        <div className="card">
          <div className="card-body">
            <h5 className="card-title">Jobs</h5>
            <div className="row">
              <div className="col text-center">
                <h1>{stats.job.runningJobs}</h1>
                Running
              </div>
              <div className="col text-center">
                <h1>{stats.job.pendingJobs}</h1>
                Pending
              </div>
            </div>
          </div>
        </div>
        <div className="card">
          <div className="card-body">
            <h5 className="card-title">Cores</h5>
            <div className="row">
              <div className="col text-center">
                <h1>{stats.flexlet.busyCores}</h1>
                Busy
              </div>
              <div className="col text-center">
                <h1>{stats.flexlet.idleCores}</h1>
                Idle
              </div>
            </div>
          </div>
        </div>
        <div className="card">
          <div className="card-body">
            <h5 className="card-title">Flexlets</h5>
            <div className="row">
              <div className="col text-center">
                <h1>{stats.flexlet.onlineFlexlets}</h1>
                Online
              </div>
              <div className="col text-center">
                <h1>{stats.flexlet.offlineFlexlets}</h1>
                Offline
              </div>
            </div>
          </div>
        </div>
      </div>
  );
}

export default function FrontPage() {
  const client = useFlexClient();
  const [stats, setStats] = useState<Stats | undefined>(undefined);
  useEffect(() => {
    (async () => {
      const stats = await client.getStats();
      setStats(stats);
    })();
  }, [client, setStats]);

  if (stats === undefined) {
    return <div>Loading...</div>;
  }

  return (
      <div>
        <h1>
          System Status
        </h1>
        <StatsCardGroup stats={stats}/>
      </div>
  );
}

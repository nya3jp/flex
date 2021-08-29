import React from 'react';
import {HashRouter as Router, Link, Route, Switch} from 'react-router-dom';
import FrontPage from './FrontPage';
import JobsPage from './JobsPage';
import FlexletsPage from './FlexletsPage';
import JobPage from './JobPage';

export default function App() {
  return (
      <Router>
        <div className="container d-flex">
          <div className="d-flex flex-column flex-shrink-0 p-3"
               style={{width: '200px'}}>
            <Link to="/"
                  className="d-flex align-items-center mb-3 mb-md-0 me-md-auto link-dark text-decoration-none">
              <span className="fs-4">Flex</span>
            </Link>

            <hr/>

            <ul className="nav nav-pills flex-column">
              <li className="nav-item">
                <Switch>
                  <Route path="/" exact>
                    <Link to="/" className="nav-link active">
                      Home
                    </Link>
                  </Route>
                  <Route>
                    <Link to="/" className="nav-link link-dark">
                      Home
                    </Link>
                  </Route>
                </Switch>
              </li>
              <li>
                <Switch>
                  <Route path="/jobs/">
                    <Link to="/jobs/" className="nav-link active">
                      Jobs
                    </Link>
                  </Route>
                  <Route>
                    <Link to="/jobs/" className="nav-link link-dark">
                      Jobs
                    </Link>
                  </Route>
                </Switch>
              </li>
              <li>
                <Switch>
                  <Route path="/flexlets/">
                    <Link to="/flexlets/" className="nav-link active">
                      Flexlets
                    </Link>
                  </Route>
                  <Route>
                    <Link to="/flexlets/" className="nav-link link-dark">
                      Flexlets
                    </Link>
                  </Route>
                </Switch>
              </li>
            </ul>
          </div>

          <main role="main" className="p-4 flex-grow-1">
            <Switch>
              <Route path="/flexlets/">
                <FlexletsPage/>
              </Route>
              <Route path="/jobs/:id">
                <JobPage/>
              </Route>
              <Route path="/jobs/" exact>
                <JobsPage/>
              </Route>
              <Route path="/" exact>
                <FrontPage/>
              </Route>
            </Switch>
          </main>
        </div>
      </Router>
  );
}

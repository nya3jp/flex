import React from 'react';
import {BrowserRouter as Router, Link, Route, Switch} from 'react-router-dom';
import FrontPage from './FrontPage';
import JobsPage from './JobsPage';
import {flexUrl} from './client';
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
            <hr/>
            <ul className="nav nav-pills flex-column">
              <li>
                <a href={flexUrl}
                   className="nav-link link-dark"
                   target="_blank" rel="noopener noreferrer">
                  Flexhub
                  <svg xmlns="http://www.w3.org/2000/svg"
                       viewBox="0 0 16 16" width="16"
                       height="16">
                    <path fillRule="evenodd"
                          d="M10.604 1h4.146a.25.25 0 01.25.25v4.146a.25.25 0 01-.427.177L13.03 4.03 9.28 7.78a.75.75 0 01-1.06-1.06l3.75-3.75-1.543-1.543A.25.25 0 0110.604 1zM3.75 2A1.75 1.75 0 002 3.75v8.5c0 .966.784 1.75 1.75 1.75h8.5A1.75 1.75 0 0014 12.25v-3.5a.75.75 0 00-1.5 0v3.5a.25.25 0 01-.25.25h-8.5a.25.25 0 01-.25-.25v-8.5a.25.25 0 01.25-.25h3.5a.75.75 0 000-1.5h-3.5z"></path>
                  </svg>
                </a>
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

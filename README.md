# Flex

Flex is a simple workload distribution system.

## Usage

```
# Install Flex CLI.
go install github.com/nya3jp/flex/cmd/flex

# Configure Flexhub endpoint and password.
flex configure

# Run a command.
flex run echo 'Hello, world!'
```

## Synopsis

```
# Upload a binary file and execute it.
flex run -f path/to/some './some --flag1 --flag2'

# Upload a binary file as a package, and tag it.
flex package create -f path/to/some -t somebin

# Enqueue a job without waiting for its completion.
flex job create -p somebin './some --flag1 --flag2'

# Print a list of finished jobs.
flex job list --state=finished

# Print details of the job whose ID is 123.
flex job info 123
```

## Tips

- Many commands accept `--json` option that prints outputs in JSON.

# labeller
A simple utility for creating and associating git labels

## Available Commands

### create
Create adds all labels specified in the provided JSON file.
The JSON file expects a format including name, color, and description.

### label
Label labels Github PRs by matching the PR title to a predefined list of tags.
The tag is associated with a PR label, which is then applied to the PR.


## Usage
To use labeller, run the following commands
```go
go get -u github.com/Rob0h/labeller
dep ensure
go build
```

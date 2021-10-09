# tfmodref
`tfmodref` is a CLI utility for working with terraform or terragrunt files which use modules stored in semantically versioned git repositories.


## Commands
### `list`
The list command (`tfmodref --list`) can be used to list local versions of modules in one or more files, and also retrieve the latest version in the repostiroy.

#### Usage
To list just local versions in the current directory and below:

`tfmodref --list`

To list local and the latest remote version in the current directory and below:

`tfmodref --list --remote`

To list local and the latest remote versions in a specific file:

`tfmodref --path a/path/to/a/file.tf --list --remote`

To list local versions in a different directory tree:

`tfmodref --path some/other/folder --list`

### `update`
The update command can be used to update the version('s) contained in file('s).

#### Usage
To update all versions in the current folder and below to the latest version:

`tfmodref update --latest`

To see what would happen, without making any changes, when performing an update:

`tfmodref update --latest --dry-run`

To update all modules to a specific version within a file:

`tfmodref update --version v0.1.0`

To update all versions to the latest available version in the remote repository within a constraint:

`tfmodref update --constraint ">0.5.0 < 2.0.x"`

## Contributing
Contributors are very welcome, people work with terraform and modules in many different ways, so please feel free to add any features or fixes you like.

## To do
- [ ] Support making changes in a git branch (auto branching)
- [ ] Support targeting a specific module
- [ ] Add tests
- [ ] Break apart update and list command wall of code
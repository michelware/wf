# ⚡ WF

**Your project's tasks, one line at a time. One binary. Plain-text `.wf` files. No YAML. No magic.**

**WF** is a minimalist workflow runner written in Go. You write your build, install and deploy steps in `.wf` files, one command per line, grouped in named sections. `wf deploy` runs the `deploy` section from top to bottom, shows every command in a readable tree with its duration, and stops at the first failure.

WF is built on a strong philosophy: **No hidden magic.** There are no conditions, no loops, no functions, no templating. Commands are not passed through a shell: what you write is what runs. A `.wf` file reads like a checklist, and anyone on your team can understand it in ten seconds.

Like a `Makefile` or a `justfile`, but with nothing to learn.

**[Download the latest release](https://github.com/michelware/wf/releases/latest)**

---

## 💾 Install

WF ships as **5 standalone binaries**. Download the one matching your platform from the [latest release](https://github.com/michelware/wf/releases/latest):

| Platform | File |
| --- | --- |
| 🐧 Linux x64 | `wf-linux-amd64` |
| 🐧 Linux ARM64 | `wf-linux-arm64` |
| 🍎 macOS Intel | `wf-macos-amd64` |
| 🍎 macOS Apple Silicon (M1/M2/M3…) | `wf-macos-arm64` |
| 🪟 Windows x64 | `wf-windows-amd64.exe` |

**Linux / macOS**: make it executable and put it in your `PATH` (replace the file name with yours):

```bash
chmod +x wf-linux-amd64
sudo mv wf-linux-amd64 /usr/local/bin/wf
wf --version
```

**Windows**: rename `wf-windows-amd64.exe` to `wf.exe` and place it in a folder listed in your `PATH` (or directly at your project root).

---

## ⚡ Why WF? (The "Plus")

### 📝 Nothing to Learn

A section is `[name]`. A command is one line. Lines starting with `#` are comments. That is the whole syntax.

### 🌳 Readable Output

Every command is printed before it runs, its output is framed, and its duration is shown when it ends. Sub-workflows are indented like a tree, so you always know which step is running and which one failed.

### 🛑 Stops at the First Failure

If a command fails, WF stops immediately with exit code `1`. No half-finished deploy that keeps going after an error.

### 🧰 Native File Commands

`mkdir`, `touch`, `write`, `copy`, `move`, `remove` and `set_permissions` are built into WF, written in Go. They do not depend on the tools installed on the machine.

### 🧙‍♂️ No Hidden Magic

No shell in between: `run composer install` runs `composer` with the argument `install`, nothing else. Quotes group words like in any terminal; pipes, globs and `$VAR` are plain text. Need shell features? Put them in a script and `run sh build.sh`.

### 🔍 Readable

The whole tool is a single Go file you can read and audit yourself.

---

## 🆚 Comparison

| | Shell script | Make | just | Task | **WF** |
| --- | --- | --- | --- | --- | --- |
| **Install** | Bash or PowerShell | `make` | Single binary | Single binary | **Single Go binary** |
| **File** | `.sh` / `.ps1` | `Makefile` | `justfile` | `Taskfile.yml` | **`.wf` files** |
| **Syntax** | A programming language | Targets, tabs, make functions | Recipes, parameters, expressions | YAML, Go templates | **One command per line** |
| **Conditions & loops** | Yes | Yes | Yes | Yes | **No, by design** |
| **Output** | Raw | Raw | Raw | Raw or prefixed | **Tree with a timer per command** |

---

## 🚀 Quick Start (KISS)

### 1. Write (`deploy.wf`)

Create a `.wf` file at the root of your project. Each `[section]` becomes a command; the comment after it becomes its description in `wf` help.

```ini
[build] # Build the application
echo "Building the application..."
run npm install
run npm run build

[permissions] # Set the rights on the output folder
set_permissions dist 0755

[deploy] # Build, set permissions and start the containers
set ENV=production
echo "Deploying in ${ENV}..."
wf build
wf permissions
docker_compose up -d
notify_success "Deployment done!"
```

### 2. Run

* **List your workflows** (big `WF` logo, then every section with its comment):

```bash
wf
```

* **Run a workflow**:

```bash
wf deploy
```

WF reads every `.wf` file in the current folder, finds the `deploy` section and runs it from top to bottom. `wf build` and `wf permissions` run as sub-workflows, indented in the output.

### 3. Read the output

```text
🚀 WORKFLOW: DEPLOY

 INFO  Variable ENV set to production
  Deploying in production...

  ╭── 📦 Sub-Workflow: build
    Building the application...

  ╭─ ▶ npm install
  │  added 312 packages in 4s
  ╰─ ✔ 4.2s
  ...
  ╰── ✔ Completed in 9.8s

 SUCCESS  Deployment done!
 SUCCESS  ✨ Workflow 'deploy' completed in 12.4s.
```

---

## 🛠️ Commands

Command keywords are case-insensitive: `set`, `SET` and `Set` are the same. Commands run in a workflow only; the CLI itself only takes a section name.

| Command | What it does |
| --- | --- |
| `run <cmd> <args...>` | Runs a program **directly, without a shell**. Arguments are split on spaces, and `"..."` or `'...'` group words like in a terminal: `--option="toto tata"` is one argument. Pipes, redirections, globs and `$VAR` are not interpreted. A non-zero exit code stops WF. |
| `docker_compose <args...>` | Runs `docker compose <args...>`, or `docker-compose <args...>` if the Compose plugin is not available. |
| `wf <section>` | Runs another section as a sub-workflow. Stops WF if the section does not exist. |
| `require <program...>` | Checks that each program is in the `PATH`. Stops WF with a clear message if one is missing. |
| `set NAME=value` | Sets a WF variable, used as `${NAME}` on the next lines. It is **not** exported to the environment of `run` commands. |
| `load_env <file>` | Loads every `NAME=value` line of a `.env` file as WF variables. |
| `mkdir <dir>` | Creates a folder and its parents. Skipped if it already exists. |
| `touch <file>` | Creates an empty file. Skipped if it already exists. The parent folder must exist. |
| `write <file> <text>` | Appends one line to a file, created if needed. The parent folder must exist. |
| `copy <src> <dst>` (or `cp`) | Copies **one file** to the full destination path. Skipped if `<dst>` already exists. |
| `move <src> <dst>` | Moves or renames a file or folder. Skipped if `<dst>` already exists. |
| `remove <path>` | Deletes a file, or a folder and everything inside it. Skipped if it does not exist. |
| `set_permissions <path> <mode>` | Applies an octal mode (`0755`, `644`…) to a file, or recursively to a folder and everything inside it (files included). |
| `sleep <seconds>` | Waits a whole number of seconds. |
| `confirm "<question>"` | Asks the question. Anything but `y` or `yes` stops WF with exit code `1`. |
| `echo <msg>` / `notify <msg>` | Prints a message. Surrounding double quotes are removed. |
| `notify_success`, `notify_info`, `notify_warning`, `notify_error <msg>` | Prints a coloured, labelled message in the terminal. `notify_error` does not stop the workflow. |
| `sync_time` | Prints the current system time. It does not change the clock. |
| `exit [code]` | Stops WF immediately, including the parent workflows, with the given exit code (`0` by default). |

Any other first word stops WF with `Unknown command or invalid syntax`. WF checks each line when it reaches it, so the lines before have already run.

---

## 🔤 Variables

```ini
load_env .env
set APP=my-app
echo "Deploying ${APP} from ${CURRENT_PATH} on ${IP_LOCAL}"
write .env.local APP_SECRET=${GENERATE_SECRET}
```

| Variable | Value |
| --- | --- |
| `${CURRENT_PATH}` | The folder where `wf` was started. |
| `${IP_LOCAL}` | The first non-loopback IPv4 address of the machine. |
| `${GENERATE_SECRET}` | 64 random hexadecimal characters. A new value on every line. |
| `${NAME}` | Any variable defined with `set NAME=value` or loaded with `load_env`. |

Variables are shared by all workflows of a run: a variable set in a sub-workflow is visible after it returns. An unknown `${NAME}` is left as is.

---

## 📖 Things to know

* **Section names must be unique** across all the `.wf` files of the folder, and inside each file. Otherwise WF refuses to start (`duplicated subcommand name`).
* Lines written before the first section form a section called `main` (`wf main`).
* `wf` lists the sections in the order they are written, file by file in alphabetical order.
* Comments must be on their own line: `run ls # list` passes `#` and `list` to `ls`.
* `remove` refuses to delete the current folder or a disk root.
* `confirm` with no one to answer (CI, piped input) stops WF: never leave one in a workflow your CI runs.
* Section names are case-sensitive on the command line: `[Deploy]` runs with `wf Deploy`.
* `wf unknown` on the command line prints `No help topic for 'unknown'` and exits with code `3`.

---

## 🚫 What WF Will Never Do

Add conditions, loops or functions, run steps in parallel, hide a shell behind your commands, load plugins, or require a YAML file.

---

## 📄 Licence

WF is distributed under the **AGPL-3.0** license. See the [LICENSE](LICENSE) file.

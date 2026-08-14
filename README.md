# samp-injector

`samp-injector.exe` launches a caller-selected `gta_sa.exe`, waits for a startup module, loads one DLL, and stays alive until GTA exits.

## Build

Build the 32-bit Windows executable without CGO:

```sh
GOOS=windows GOARCH=386 CGO_ENABLED=0 go build -trimpath -o dist/samp-injector-windows-386.exe ./cmd/samp-injector
```

The same build is available through `task build`.

## Run

```text
samp-injector.exe --game "C:\Games\GTA San Andreas\gta_sa.exe" --dll "C:\Games\GTA San Andreas\samp.dll" -- -c -n Player -h 127.0.0.1 -p 7777
```

Everything after `--` is forwarded to GTA. The injector does not interpret those arguments.

Options:

```text
--game <path>                 GTA executable to launch (required)
--dll <path>                  DLL to inject (required)
--cwd <path>                  working directory; defaults to the game directory
--wait-module <filename>      module to wait for; default: vorbisFile.dll
--no-wait-module              inject without waiting for a module
--wait-timeout <duration>     readiness timeout; default: 30s
```

The readiness comparison uses the module basename and is case-insensitive. The wait is bounded.

## Ownership

GTA is created suspended, assigned to a Windows Job Object, and resumed only after ownership is established. The Job Object uses kill-on-close. If the injector exits unexpectedly and Windows supports that limit, GTA is terminated with it. The injector remains alive after injection and exits when GTA exits.

The injector-specific exit codes are:

```text
2  invalid arguments or supplied paths
3  process creation or lifecycle ownership failure
4  readiness timeout or GTA exited before readiness
5  DLL injection failure
6  other Win32 or process-status failure
```

After a successfully injected session, GTA's exit code is returned.

## Wine and Proton

The binary is Windows-only. The caller must provide the Wine or Proton environment and invoke the executable there. The injector does not discover prefixes, installations, Steam, or Proton.

Failures are written to stderr. Successful operation is quiet.

## Testing

Run the portable tests with:

```sh
go test -race -shuffle=on -count=1 ./...
```

Win32 behavior needs a Windows-compatible environment and a 32-bit target fixture. The release binary does not need a C or C++ runtime.

## Development

Install Go 1.25.x, Task 3.52.0, gofumpt 0.11.0, and golangci-lint 2.12.2. Use these commands during development:

```sh
task fmt
task check
task test
task build
task ci
```

`task check` runs formatting, vet, lint, and the portable tests. The vet and lint steps also check the Windows 386 packages. It does not run Win32 runtime or DLL-injection tests.
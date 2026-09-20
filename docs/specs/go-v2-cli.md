# Opt-in schema 2.0 CLI

This increment connects the [native v2 coordinator](go-v2-native-assembly.md) to the CLI for [Issue #17](https://github.com/toddwbucy/Aletharsis/issues/17). It does not switch the default schema or enable an external detector.

## Usage and compatibility

```sh
aletharsis audit source.txt --schema-version 2.0
aletharsis audit source.txt --schema-version=2.0 --json
aletharsis unicode source.txt --schema-version 2.0 --output new-report.json
```

All four existing commands accept separate or equals-form schema selection. Only `1.0` and `2.0` are accepted; missing, empty and unknown versions fail before acquisition. `--` still ends option parsing. Explicit `1.0` follows the unchanged legacy output path. The default remains 1.0; help adds the new option.

The v2 JSON path delivers the validated canonical UTF-8 bytes with no trailing newline. This is machine-readable evidence, not terminal-safe display: consumers must treat strings as untrusted and hash the exact received bytes before interpretation. Existing v1 JSON keeps its prior ASCII escaping and formatting. Output files always contain JSON, even without `--json`.

## Coverage and exits

Human output places capability and execution coverage before the finding summary. It reports mechanism, availability, participation, execution state, reason codes, requested scope and analyzed/excluded scope counts. Verbose mode includes the full scope/exclusion records. Typed diagnostics remain visible even with no findings. Terminal display escapes source paths and diagnostic strings.

C2PA extraction/verification and Anthropic watermark detection are disabled and unavailable. They produce neither positive nor negative results. Optional disabled declarations do not make an otherwise completed structural audit fail. Failed, canceled or incomplete required work retains its operational status and exits 4; completed audits retain the existing selected-view severity exits 0–3. Zero findings never establishes the absence of a watermark.

Finding views retain the full evidence, execution and result graph. A filtered-out observation still belongs to its positive structural result. Presentation does not authorize deletion, suppress another mechanism, or manufacture statistical watermark spans. Frontend/reveal consumers must honor the same distinction under EC-001/EC-002; no reveal or UI implementation is introduced here.

## Output and interruption

Exclusive creation uses mode 0600 and refuses existing destinations, including source paths, symlinks and hard links. Short writes count as failures. Write or close failure attempts to remove the newly created partial output; cleanup failure is ignored, so a failed command must never be treated as proof of successful delivery. Stdout may already contain partial bytes when an output failure occurs.

V2 output errors use stable stderr codes `output.create_failed`, `output.write_failed`, and `output.close_failed`, and exit 4. Assembly budget rejection reports `execution.resource_limit`; other assembly errors report `execution.failed`, without dumping internal errors or document content. No output file is created before a complete report is validated. Verbose structured logs go to stderr; failure to write them fails the command.

Ctrl-C requests cooperative cancellation at native operation boundaries. It cannot preempt a running analyzer or guarantee a hard timeout. Completed evidence is retained when an interruption is observed. Default source and report budgets are documented in the coordinator specification; they are not hard memory ceilings.

## Validation and remaining gates

Compiled CLI tests check all 16 public input fixtures against the independent schema/semantic oracle and legacy payloads, both option forms, deterministic bytes, views, verbose separation, source bytes/timestamps, unavailable coverage, typed acquisition failure, output permissions/overwrite refusal and resource rejection without a partial file. Unit tests exercise short writes, write/close errors, cleanup and all eight synthetic coverage fixtures in the human renderer.

Consumer integration, measured large-report/resource acceptance and third-party review remain necessary to complete #17. A later default-schema switch requires explicit release approval. This CLI introduces no C2PA verifier, statistical heuristic, network request, structured-format parser or remediation operation.

# ECMAScript number oracle

`ecmascript-numbers.jsonl` is independently generated test data, not upstream
source code. It records 256 deterministic finite binary64 inputs and V8's
`JSON.stringify` output. The seed and arithmetic below define input identity;
hexadecimal `bits` is the exact IEEE-754 representation, not a decimal approximation.

Generated with Node **v26.8.1**, V8 **14.6.202.34-node.28**, on 2026-09-20.
SHA-256: `f01c53341d8b63af4566f24747a7bf6a9889200a627bd312068f2ef26fd37402`.
Go tests pin the digest and count. Node is not required in CI or production.
Updating this oracle requires reviewing differences and recording the new runtime.

Reproduce by running this original project-owned JavaScript with that Node version
and redirecting stdout to a temporary file for comparison (do not overwrite the
accepted fixture without review):

```javascript
let state = 1729n;
let count = 0;
while (count < 256) {
  state = BigInt.asUintN(64, state * 6364136223846793005n + 1442695040888963407n);
  const bytes = Buffer.alloc(8);
  bytes.writeBigUInt64BE(state);
  const value = bytes.readDoubleBE();
  if (!Number.isFinite(value)) continue;
  process.stdout.write(JSON.stringify({bits: state.toString(16).padStart(16, '0'), canonical: JSON.stringify(value)}) + '\n');
  count++;
}
```

These samples complement [RFC 8785 Appendix B](https://www.rfc-editor.org/rfc/rfc8785#appendix-B)
and the existing portable vectors. They are interoperability evidence, not an
exhaustive proof over all binary64 values. Report-coordinate safe-integer rules
remain separate from general JCS number serialization.

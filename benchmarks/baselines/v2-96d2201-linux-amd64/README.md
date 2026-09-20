# V2 resource probe after budget fixes

Clean candidate `96d2201` uses the same corpus and single-sample procedure as the [initial probe](../v2-097b35e-linux-amd64/README.md). This is a local diagnostic comparison, not a statistically established service guarantee. Exact binary, harness, corpus, compiler and machine identities remain in `results.json`.

| Case | Source bytes | Outcome | Report bytes | Wall seconds | Peak RSS KiB |
| --- | ---: | --- | ---: | ---: | ---: |
| ascii | 65,536 | valid report | 469,437 | 0.125 | 25,200 |
| multilingual | 65,536 | valid report | 438,183 | 0.156 | 27,488 |
| source | 65,536 | valid report | 536,076 | 0.154 | 29,624 |
| zero_width | 65,536 | valid report | 2,324,885 | 1.061 | 95,556 |
| periodic | 65,536 | valid report | 2,216,905 | 0.886 | 93,232 |
| combining | 65,536 | valid report | 660,347 | 0.446 | 31,688 |
| ascii | 1,048,576 | valid report | 8,365,131 | 1.400 | 181,076 |
| multilingual | 1,048,576 | valid report | 7,399,756 | 1.751 | 190,560 |
| source | 1,048,576 | valid report | 9,445,906 | 1.793 | 202,740 |
| zero_width | 1,048,576 | budget rejection | 0 | 4.461 | 231,748 |
| periodic | 1,048,576 | budget rejection | 0 | 4.509 | 223,596 |
| combining | 1,048,576 | valid report | 11,930,248 | 5.990 | 253,500 |
| ascii | 8,388,608 | budget rejection | 0 | 0.216 | 223,004 |
| multilingual | 8,388,608 | budget rejection | 0 | 0.190 | 123,796 |
| source | 8,388,608 | budget rejection | 0 | 0.228 | 221,452 |
| zero_width | 8,388,608 | budget rejection | 0 | 0.161 | 119,100 |
| periodic | 8,388,608 | budget rejection | 0 | 0.185 | 181,152 |
| combining | 8,388,608 | budget rejection | 0 | 0.208 | 158,864 |

All six 64 KiB cases now emit valid independently checked reports. ASCII, multilingual, source-like and combining inputs also complete at 1 MiB. The 1 MiB zero-width case reaches the whole-report node check; the periodic case exceeds the report byte budget. Eight MiB inputs cannot fit their mandatory text/boundary arrays under the 16 MiB report cap and are rejected after parsing, before analyzers and anchor assembly. No evidence is truncated.

The before/after RSS reduction for 8 MiB ASCII is attributable to earlier rejection rather than improved completed-audit throughput. Smaller valid dense reports now incur real validation work that the earlier candidate avoided by rejecting them. RSS excludes the parent schema validator and browser, and these limits remain validation budgets rather than a hard memory ceiling.

Per-anchor and selection budgets remain unchanged. The report node allowance is now 4,194,304 (from 1,000,000), accommodating million-entry boundary arrays under the unchanged 16 MiB byte cap. Finding ordering keys use report budgets; many disjoint occurrences are partitioned across bounded anchors. Historical findings, coordinates and v1 reports are retained.

These measurements support review of the corrected opt-in implementation. They do not authorize a default-version switch, prove universal worst-case behavior or complete the browser viewer spike.

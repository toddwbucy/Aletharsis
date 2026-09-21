// Execute the unchanged pinned scan event handler with an inert DOM facade.
// This is a detector-function probe, not a browser conformance/security test.
'use strict';
const fs = require('node:fs');
const vm = require('node:vm');
class Element {
  constructor() { this.children = []; this.events = {}; this.value = ''; this.style = {}; this.hidden = null; this._text = ''; }
  set textContent(v) { this._text = String(v); this.children = []; }
  get textContent() { return this._text + this.children.map(c => c.textContent).join(''); }
  appendChild(c) { this.children.push(c); return c; }
  addEventListener(event, callback) { this.events[event] = callback; }
  setAttribute() {}
}
const elements = new Map();
const get = id => { if (!elements.has(id)) elements.set(id, new Element()); return elements.get(id); };
const document = {getElementById:get, querySelectorAll:()=>[], addEventListener:()=>{},
  createElement:()=>new Element(), createTextNode:text=>{ const n = new Element(); n.textContent=text; n.isText=true; return n; }};
const html = fs.readFileSync('/upstream/index.html','utf8');
const scripts = [...html.matchAll(/<script>([\s\S]*?)<\/script>/g)];
if (scripts.length !== 1) throw new Error('unexpected upstream executable script count');
const context = vm.createContext({document, TextEncoder, TextDecoder});
vm.runInContext(scripts[0][1], context, {timeout:1000});
const request = JSON.parse(fs.readFileSync(0,'utf8'));
get('scanInput').value = request.text;
vm.runInContext("document.getElementById('btnScan').events.click()", context, {timeout:1000});
const input = Array.from(request.text);
const observations = [];
let scalar = 0;
for (const child of get('scanViz').children) {
  if (child.isText) { scalar += Array.from(child.textContent).length; continue; }
  const cp = child.textContent.replace(/^·/, '');
  if (scalar >= input.length) throw new Error('visual map overran input');
  if (cp !== 'U+'+input[scalar].codePointAt(0).toString(16).toUpperCase().padStart(4,'0')) throw new Error('visual position mismatch');
  observations.push({scalar, code_point:cp, native_category:child.className});
  scalar++;
}
if (scalar !== input.length) throw new Error('visual map incomplete');
const verdict = get('scanVerdict').textContent;
const hidden = get('scanSecret').hidden; // null means the facade observed no assignment.
process.stdout.write(JSON.stringify({position_units:'Unicode scalar indices reconstructed from scanViz child order',
  observations, verdict, scan_status:verdict === '' && observations.length === 0 ? 'no_verdict_emitted' : 'completed',
  secret_visible:typeof hidden === 'boolean' ? !hidden : null,
  secret_text:get('scanSecret').textContent, input_unchanged:get('scanInput').value === request.text})+'\n');

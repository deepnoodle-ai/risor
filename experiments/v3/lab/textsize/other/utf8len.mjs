// What it costs JavaScript to know the UTF-8 length of a string it only reads.
// Compare with s.length, which is the weight of 04-runtime.md A.5 and is free.
// Run: node utf8len.mjs

function utf8Length(s) {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c < 0x80) n += 1;
    else if (c < 0x800) n += 2;
    else if (c >= 0xd800 && c <= 0xdbff) { n += 4; i++; }
    else n += 3;
  }
  return n;
}

function codePoints(s) {
  let n = 0;
  for (let i = 0; i < s.length; i++) {
    const c = s.charCodeAt(i);
    if (c >= 0xd800 && c <= 0xdbff) i++;
    n++;
  }
  return n;
}

const encoder = new TextEncoder();
const scratch = new Uint8Array(3 * 4096);

const cases = {
  "utf8 loop": utf8Length,
  "code points loop": codePoints,
  "TextEncoder.encodeInto": (s) => encoder.encodeInto(s, scratch).written,
  "Buffer.byteLength (Node only)": (s) => Buffer.byteLength(s, "utf8"),
  "length (weight)": (s) => s.length,
};

function make(units, mixed) {
  let s = "";
  for (let i = 0; s.length < units; i++) s += mixed && i % 3 === 0 ? "é" : "a";
  // Flatten the rope the concatenation built, so the scan reads flat storage.
  return JSON.parse(JSON.stringify(s));
}

const inputs = [
  ["16 ASCII", make(16, false)],
  ["256 ASCII", make(256, false)],
  ["4,096 ASCII", make(4096, false)],
  ["4,096, one third non-ASCII", make(4096, true)],
];

function bench(fn, s) {
  let sink = 0;
  let iters = 1000;
  for (;;) {
    const t0 = process.hrtime.bigint();
    for (let i = 0; i < iters; i++) sink += fn(s);
    const ns = Number(process.hrtime.bigint() - t0);
    if (ns > 2e8) return [ns / iters, sink];
    iters *= 4;
  }
}

for (const [name, fn] of Object.entries(cases)) {
  for (const [, s] of inputs) bench(fn, s); // warm up
  const row = inputs.map(([, s]) => {
    const runs = [];
    for (let r = 0; r < 5; r++) runs.push(bench(fn, s)[0]);
    runs.sort((a, b) => a - b);
    return runs[2].toFixed(1) + " ns";
  });
  console.log(name.padEnd(32), row.join("  |  "));
}
console.log("columns:", inputs.map(([n]) => n).join("  |  "));

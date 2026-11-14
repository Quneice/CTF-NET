/**
 * 移位变换工具 —— 10 个端到端实例(node 20.6+ 直跑,仓库根目录:node scripts/shift_examples.mjs)
 *
 * 每个实例:构造明文(flag 格式)→ 按层链加密 → 用工具能力(单层富爆破 / 多层链爆破 / 整体求逆)还原 → 断言。
 * 覆盖:常数/递增/字母域/可打印域/单字节 XOR/循环密钥前缀攻击/多层组合/预算闸门/回环/不可逆判定。
 */
const lib = await import('../frontend/src/lib/shift.ts');
const { applyLayers, applyShift, invertLayers, bruteForceShift, bruteForceLayers, describeShift } = lib;

let pass = 0;
let fail = 0;
const esc = (s) => Array.from(s, (c) => (c.charCodeAt(0) >= 0x20 && c.charCodeAt(0) <= 0x7e ? c : `\\x${(c.charCodeAt(0) & 0xff).toString(16).padStart(2, '0')}`)).join('');
const hex = (s) => Array.from(s, (c) => (c.charCodeAt(0) & 0xff).toString(16).padStart(2, '0')).join(' ');

function run(no, title, plain, encLayers, lines, check) {
  const cipher = applyLayers(plain, encLayers);
  console.log(`\n例 ${no}|${title}`);
  console.log(`  明文   : ${esc(plain)}`);
  console.log(`  加密链 : ${encLayers.map(describeShift).join(' → ')}`);
  console.log(`  密文   : ${esc(cipher)}  [${hex(cipher)}]`);
  for (const l of lines) console.log('  ' + l);
  try {
    const ok = check(cipher);
    if (ok) {
      pass++;
      console.log('  结果   : ✓ PASS');
    } else {
      fail++;
      console.log('  结果   : ✗ FAIL(断言未通过)');
      process.exitCode = 1;
    }
  } catch (e) {
    fail++;
    console.log('  结果   : ✗ FAIL(' + e.message + ')');
    process.exitCode = 1;
  }
}

// —— 例 1:单层常数移位,前缀爆破 ——
run(1, '常数移位爆破', 'flag{caesar_shift}', [{ mode: 'const', domain: 'byte', n: -7 }], ['爆破   : 单层富路径,格式 flag{'], (cipher) => {
  const hits = bruteForceShift(cipher, 'flag{', false);
  return hits.some((h) => h.plain === 'flag{caesar_shift}' && h.opts.n === 7);
});

// —— 例 2:单层递增移位(第 0 位 +2,第 1 位 +5,第 2 位 +8 …)——
run(2, '递增移位爆破', 'flag{incremental_ladder}', [{ mode: 'inc', domain: 'byte', n: 2, s: 3 }], ['参数   : b=2 s=3 → 第 i 位 +(2+3i)'], (cipher) => {
  const hits = bruteForceShift(cipher, 'flag{', false);
  return hits.some((h) => h.plain === 'flag{incremental_ladder}' && h.opts.n === -2 && h.opts.s === -3);
});

// —— 例 3:字母域凯撒(保大小写,非字母不动)——
run(3, '字母域凯撒', 'flag{Alpha_Keep_Case}', [{ mode: 'const', domain: 'alpha', n: 5 }], ['观察   : f→k、A→F、_ 与 { } 原样'], (cipher) => {
  const sample = applyShift('aBz9', { mode: 'const', domain: 'alpha', n: 2 }) === 'cDb9';
  const hits = bruteForceShift(cipher, 'flag{', false);
  return sample && hits.some((h) => h.plain === 'flag{Alpha_Keep_Case}' && h.opts.domain === 'alpha');
});

// —— 例 4:单字节 XOR,256 穷举 ——
run(4, '单字节 XOR 爆破', 'flag{single_byte_xor}', [{ mode: 'xor', domain: 'byte', n: 0, key: '\x37' }], ['密钥   : 0x37(单字符即单字节)'], (cipher) => {
  const hits = bruteForceShift(cipher, 'flag{', false);
  return hits.some((h) => h.plain === 'flag{single_byte_xor}' && h.desc.includes('37'));
});

// —— 例 5:8 字节循环密钥,已知前缀攻击直接解出密钥 ——
run(5, '循环密钥前缀攻击', 'flag{2026_repeating_key_attack!}', [{ mode: 'xor', domain: 'byte', n: 0, key: 'UniqK3y!' }], ['格式   : flag{2026}(10 字符 ≥ 密钥长 8,前缀攻击可解)'], (cipher) => {
  const hits = bruteForceShift(cipher, 'flag{2026', false);
  return hits.some((h) => h.plain === 'flag{2026_repeating_key_attack!}' && h.desc.includes('UniqK3y!'));
});

// —— 例 6:两层(递增 + XOR),多层链爆破还原加密参数 ——
run(6, '两层链爆破', 'flag{two_layer_chain}', [
  { mode: 'inc', domain: 'byte', n: 3, s: 2 },
  { mode: 'xor', domain: 'byte', n: 0, key: 'Q' },
], ['爆破   : 多层路径,按层结构穷举参数(解码=逆序取逆)'], (cipher) => {
  const r = bruteForceLayers(cipher, 'flag{', false, [
    { mode: 'inc', domain: 'byte', n: 0, s: 0 },
    { mode: 'xor', domain: 'byte', n: 0, key: '' },
  ]);
  const hit = r.hits.find((h) => h.plain === 'flag{two_layer_chain}');
  return !!hit && hit.layers[0].n === 3 && hit.layers[0].s === 2 && hit.layers[1].key === 'Q';
});

// —— 例 7:三层(常数 + ROL + 字符串左旋),值/位/位置三类混合 ——
run(7, '三层混合链爆破', 'flag{three_mixed_layers}', [
  { mode: 'const', domain: 'byte', n: -5 },
  { mode: 'rol', domain: 'byte', n: 3 },
  { mode: 'rotl', domain: 'byte', n: 2 },
], ['说明   : 值移位+位循环+位置旋转三类不可交换,逆序取逆还原'], (cipher) => {
  const r = bruteForceLayers(cipher, 'flag{', false, [
    { mode: 'const', domain: 'byte', n: 0 },
    { mode: 'rol', domain: 'byte', n: 0 },
    { mode: 'rotl', domain: 'byte', n: 0 },
  ]);
  return r.hits.some((h) => h.plain === 'flag{three_mixed_layers}' && h.layers[2].n === 2 && h.layers[1].n === 3);
});

// —— 例 8:四层重链,预算闸门拒绝(高层数放弃)——
run(8, '四层超预算拒绝', 'flag{too_many_layers}', [
  { mode: 'inc', domain: 'byte', n: 1, s: 1 },
  { mode: 'inc', domain: 'byte', n: 2, s: -1 },
  { mode: 'inc', domain: 'byte', n: -3, s: 2 },
  { mode: 'xor', domain: 'byte', n: 0, key: 'W' },
], ['组合   : 85×85×85×256 = 1.58 亿 >> 预算 200 万'], (cipher) => {
  const r = bruteForceLayers(cipher, 'flag{', false, [
    { mode: 'inc', domain: 'byte', n: 0, s: 0 },
    { mode: 'inc', domain: 'byte', n: 0, s: 0 },
    { mode: 'inc', domain: 'byte', n: 0, s: 0 },
    { mode: 'xor', domain: 'byte', n: 0, key: '' },
  ]);
  return r.budgetExceeded && r.hits.length === 0 && r.combos === 85 * 85 * 85 * 256;
});

// —— 例 9:双域两层(字母环 + 可打印环),域随层结构保留 ——
run(9, '双域两层链', 'flag{two_domains}', [
  { mode: 'const', domain: 'alpha', n: 4 },
  { mode: 'inc', domain: 'printable', n: 1, s: 1 },
], ['说明   : 第 1 层字母环、第 2 层可打印环;域由用户层指定'], (cipher) => {
  const r = bruteForceLayers(cipher, 'flag{', false, [
    { mode: 'const', domain: 'alpha', n: 0 },
    { mode: 'inc', domain: 'printable', n: 0, s: 0 },
  ]);
  const hit = r.hits.find((h) => h.plain === 'flag{two_domains}');
  return !!hit && hit.layers[0].domain === 'alpha' && hit.layers[1].domain === 'printable';
});

// —— 例 10:整体求逆回环 + 有损链不可逆判定 ——
run(10, '回环与不可逆判定', 'flag{roundtrip_and_lossy}', [
  { mode: 'inc', domain: 'byte', n: 2, s: 1 },
  { mode: 'xor', domain: 'byte', n: 0, key: 'Key' },
  { mode: 'ror', domain: 'byte', n: 4 },
  { mode: 'rotr', domain: 'byte', n: 3 },
], ['回环   : invertLayers 逆序取逆 → 原文;含 SHL 的链 → null'], (cipher) => {
  const inv = invertLayers([
    { mode: 'inc', domain: 'byte', n: 2, s: 1 },
    { mode: 'xor', domain: 'byte', n: 0, key: 'Key' },
    { mode: 'ror', domain: 'byte', n: 4 },
    { mode: 'rotr', domain: 'byte', n: 3 },
  ]);
  const roundtrip = inv && applyLayers(cipher, inv) === 'flag{roundtrip_and_lossy}';
  const lossy = invertLayers([
    { mode: 'const', domain: 'byte', n: 1 },
    { mode: 'shl', domain: 'byte', n: 2 },
  ]) === null;
  return roundtrip && lossy;
});

console.log(`\n========== 汇总:${pass} 通过 / ${fail} 失败 ==========`);
if (fail > 0) process.exitCode = 1;

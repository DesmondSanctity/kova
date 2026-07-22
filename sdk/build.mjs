import { build, context } from 'esbuild';

const shared = {
 bundle: true,
 minify: true,
 sourcemap: true,
 target: ['es2020'],
};

const outputs = [
 { entryPoints: ['src/index.ts'], format: 'esm', outfile: 'dist/kova.mjs' },
 { entryPoints: ['src/index.ts'], format: 'cjs', outfile: 'dist/kova.cjs' },
 {
  entryPoints: ['src/global.ts'],
  format: 'iife',
  outfile: 'dist/kova.global.js',
 },
];

if (process.argv.includes('--watch')) {
 const ctx = await context({ ...shared, ...outputs[0] });
 await ctx.watch();
 console.log('watching…');
} else {
 await Promise.all(outputs.map((o) => build({ ...shared, ...o })));
 console.log('built dist/');
}

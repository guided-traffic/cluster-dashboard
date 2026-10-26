// Files a spec reads as text through the build's text loader:
// `import source from './file.css' with { loader: 'text' }`.
declare module '*.css' {
  const source: string;
  export default source;
}

declare module '*/theme-init.js' {
  const source: string;
  export default source;
}

/// <reference types="vite/client" />

// Declare CSS modules for side-effect imports
declare module "*.css" {
  const content: string;
  export default content;
}

// Specific declarations for @xterm/xterm CSS
declare module "@xterm/xterm/css/xterm.css" {
  const content: string;
  export default content;
}

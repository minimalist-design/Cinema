// hls.js ships no types for its light build (no subtitles, alternate audio
// or DRM), which has the same API as the full one.
declare module "hls.js/light" {
  export { default } from "hls.js";
}

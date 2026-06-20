export interface Server {
  name: string;
  host: string;
  port: number;
}

// Mirrors the Go profile.Profile (index.json on S3). Ping fields (players/status)
// are runtime-only, not part of the manifest, so they are optional.
export interface Profile {
  slug: string;
  title: string;
  version: string;
  mcVersion: string;
  desc: string;
  icon: string;
  accent: string;
  accentDim: string;
  bg: string;
  order: number;
  hidden: boolean;
  servers: Server[];
  manifestPath: string;
  players?: { online: number; max: number };
  status?: "online" | "offline";
}

export interface OptFile {
  id: string;
  name: string;
  desc: string;
  size: string;
  on: boolean;
}

export type GameState = "idle" | "fetching" | "dl" | "prep" | "playing";
export type Tab = "play" | "info" | "detail" | "settings";
export type User = { username: string; avatarUrl?: string };
export type BadgeVariant = "online" | "offline" | "pinging" | "version";
export type ButtonVariant = "primary" | "danger" | "ghost" | "icon-sq";

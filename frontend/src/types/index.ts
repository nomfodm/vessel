export interface Profile {
  id: number;
  slug: string;
  title: string;
  version: string;
  desc: string;
  players: { online: number; max: number };
  status: "online" | "offline";
  bg: string;
  accent: string;
  accentDim: string;
  icon: string;
}

export interface OptFile {
  id: string;
  name: string;
  desc: string;
  size: string;
  on: boolean;
}

export type GameState = "idle" | "fetching" | "dl" | "prep" | "playing";
export type UpdateState =
  | "check"
  | "none"
  | "found"
  | "updating"
  | "restart"
  | "error";
export type Tab = "play" | "info" | "detail";
export type User = { username: string };
export type BadgeVariant = "online" | "offline" | "pinging" | "version";
export type ButtonVariant = "primary" | "danger" | "ghost" | "icon-sq";

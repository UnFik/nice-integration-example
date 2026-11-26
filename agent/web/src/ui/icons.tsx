import type { ReactNode } from "react";

// Stroke icons on a 24px grid, drawn for this UI: each one names a thing on screen
// (percakapan, organisasi, project, tool), not a generic glyph.
const paths = {
  box: <path d="M3 7.5 12 3l9 4.5v9L12 21l-9-4.5zM3 7.5l9 4.5 9-4.5M12 12v9" />,
  org: <path d="M4 21V4h10v17M14 9h6v12M2 21h20M8 8h2M8 12h2M8 16h2" />,
  project: <path d="M3 6h7l2 2.5h9V19H3zM3 6v13" />,
  alert: <path d="M12 3 2 20h20zM12 10v4.5M12 17.2v.3" />,
  info: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 11v5.5M12 7.6v.3" />
    </>
  ),
  clock: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M12 7v5.2l3.2 2" />
    </>
  ),
  block: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M5.6 5.6l12.8 12.8" />
    </>
  ),
  offline: <path d="M4 4l16 16M8.5 8.8A8 8 0 0 0 5 11.2M12 6a11 11 0 0 1 7 2.8M9 14.6a4.5 4.5 0 0 1 4.4-.9M12 18.4v.1" />,
  gear: <path d="M4 7h10M18 7h2M4 17h2M10 17h10M14 4v6M6 14v6" />,
  check: (
    <>
      <circle cx="12" cy="12" r="9" />
      <path d="M8 12.3l2.8 2.8L16.2 9.6" />
    </>
  ),
  x: <path d="M6 6l12 12M18 6 6 18" />,
  plus: <path d="M12 5v14M5 12h14" />,
  pencil: <path d="M4 20l1-4L16.5 4.5a2 2 0 0 1 3 3L8 19zM14.5 6.5l3 3" />,
  trash: <path d="M4 7h16M9 7V4h6v3M6 7l1 13h10l1-13M10 11v5M14 11v5" />,
  copy: <path d="M9 9h11v11H9zM5 15V4h11" />,
  logout: <path d="M10 4H4v16h6M15 8l4 4-4 4M19 12H9" />,
  switch: <path d="M4 8h13l-3-3M20 16H7l3 3" />,
  chevron: <path d="M7 10l5 5 5-5" />,
  refresh: <path d="M20 12a8 8 0 1 1-2.4-5.7M20 4v5h-5" />,
  spinner: <path d="M12 3a9 9 0 1 0 9 9" />,
  chat: <path d="M4 5h16v11H9l-5 4z" />,
  send: <path d="M4 12 20 4l-6 16-3-7zM11 13l9-9" />,
  tool: <path d="M14.5 5.5a4 4 0 0 0-5 5L4 16l4 4 5.5-5.5a4 4 0 0 0 5-5l-2.5 2.5-3-1-1-3z" />,
  shield: <path d="M12 3 5 6v5c0 4.5 3 8.2 7 10 4-1.8 7-5.5 7-10V6zM9 12l2 2 4-4" />,
  token: <path d="M4 12a4 4 0 0 1 4-4h8a4 4 0 0 1 0 8H8a4 4 0 0 1-4-4zM9 12h6" />,
} satisfies Record<string, ReactNode>;

export type IconName = keyof typeof paths;

export function Icon({ name, className }: { name: IconName; className?: string }) {
  return (
    <svg
      className={`icon${className ? ` ${className}` : ""}`}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.8"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
      focusable="false"
    >
      {paths[name]}
    </svg>
  );
}

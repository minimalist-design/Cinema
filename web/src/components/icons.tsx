// A few inline icons (Tabler outline style), to avoid an icon dependency.
import type { ReactNode } from "react";

function Icon({ size = 18, children }: { size?: number; children: ReactNode }) {
  return (
    <svg
      width={size}
      height={size}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2}
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {children}
    </svg>
  );
}

export const SunIcon = () => (
  <Icon>
    <circle cx="12" cy="12" r="4" />
    <path d="M3 12h1m8-9v1m8 8h1m-9 8v1M5.6 5.6l.7.7m12.1-.7-.7.7m0 11.4.7.7m-12.1-.7-.7.7" />
  </Icon>
);

export const MoonIcon = () => (
  <Icon>
    <path d="M12 3a6 6 0 0 0 9 9 9 9 0 1 1-9-9z" />
  </Icon>
);

export const SearchIcon = () => (
  <Icon size={16}>
    <circle cx="10" cy="10" r="7" />
    <path d="m21 21-6-6" />
  </Icon>
);

export const RefreshIcon = () => (
  <Icon size={16}>
    <path d="M20 11A8.1 8.1 0 0 0 4.5 9M4 5v4h4m-4 4a8.1 8.1 0 0 0 15.5 2m.5 4v-4h-4" />
  </Icon>
);

export const ArrowRightIcon = () => (
  <Icon size={16}>
    <path d="M5 12h14m-6 6 6-6m-6-6 6 6" />
  </Icon>
);

export const AlertIcon = () => (
  <Icon>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 8v4m0 4h.01" />
  </Icon>
);

export const FilmIcon = ({ size = 32 }: { size?: number }) => (
  <Icon size={size}>
    <rect x="4" y="4" width="16" height="16" rx="2" />
    <path d="M8 4v16M16 4v16M4 8h4m-4 8h4m-4-4h16m-4-4h4m-4 8h4" />
  </Icon>
);

export const SettingsIcon = () => (
  <Icon>
    <path d="M10.3 4.3c.4-1.8 3-1.8 3.4 0a1.7 1.7 0 0 0 2.6 1.1c1.5-.9 3.3.8 2.4 2.4a1.7 1.7 0 0 0 1 2.5c1.8.4 1.8 3 0 3.4a1.7 1.7 0 0 0-1 2.6c.9 1.5-.9 3.3-2.4 2.4a1.7 1.7 0 0 0-2.6 1c-.4 1.8-3 1.8-3.4 0a1.7 1.7 0 0 0-2.5-1c-1.6.9-3.3-.9-2.4-2.4a1.7 1.7 0 0 0-1.1-2.6c-1.8-.4-1.8-3 0-3.4a1.7 1.7 0 0 0 1.1-2.5c-.9-1.6.8-3.3 2.4-2.4 1 .6 2.3 0 2.5-1.1z" />
    <circle cx="12" cy="12" r="3" />
  </Icon>
);

export const FolderIcon = ({ size = 18 }: { size?: number }) => (
  <Icon size={size}>
    <path d="M5 4h4l3 3h7a2 2 0 0 1 2 2v8a2 2 0 0 1-2 2H5a2 2 0 0 1-2-2V6a2 2 0 0 1 2-2" />
  </Icon>
);

export const TrashIcon = () => (
  <Icon size={16}>
    <path d="M4 7h16m-10 4v6m4-6v6M5 7l1 12a2 2 0 0 0 2 2h8a2 2 0 0 0 2-2l1-12M9 7V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v3" />
  </Icon>
);

export const PlusIcon = () => (
  <Icon size={16}>
    <path d="M12 5v14m-7-7h14" />
  </Icon>
);

export const ArrowUpIcon = () => (
  <Icon size={16}>
    <path d="M12 19V5m-6 6 6-6 6 6" />
  </Icon>
);

export const HomeIcon = () => (
  <Icon size={16}>
    <path d="M5 12H3l9-9 9 9h-2M5 12v7a2 2 0 0 0 2 2h10a2 2 0 0 0 2-2v-7" />
    <path d="M9 21v-6a2 2 0 0 1 2-2h2a2 2 0 0 1 2 2v6" />
  </Icon>
);

export const LibraryIcon = () => (
  <Icon size={16}>
    <path d="M4 4h4v16H4zm6 0h4v16h-4zm6.5 1.5 3.8-1 3.7 15.4-3.8 1z" />
  </Icon>
);

export const InfoIcon = () => (
  <Icon size={16}>
    <circle cx="12" cy="12" r="9" />
    <path d="M12 8h.01M11 12h1v4h1" />
  </Icon>
);

export const CaptionsIcon = () => (
  <Icon size={16}>
    <rect x="3" y="5" width="18" height="14" rx="2" />
    <path d="M10.5 10.5a1.5 1.5 0 0 0-3 0v3a1.5 1.5 0 0 0 3 0m6-3a1.5 1.5 0 0 0-3 0v3a1.5 1.5 0 0 0 3 0" />
  </Icon>
);

export const ServerIcon = () => (
  <Icon size={16}>
    <rect x="3" y="4" width="18" height="7" rx="2" />
    <rect x="3" y="13" width="18" height="7" rx="2" />
    <path d="M7 8v.01M7 17v.01" />
  </Icon>
);

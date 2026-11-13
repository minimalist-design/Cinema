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

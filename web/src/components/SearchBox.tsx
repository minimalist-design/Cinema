import { CloseButton, TextInput, type TextInputProps } from "@mantine/core";
import { useLocation, useNavigate, useSearchParams } from "react-router-dom";
import { SearchIcon } from "./icons";

// SearchBox searches the library. The query lives in the home page's URL
// (/?q=...), so typing from any page goes to the results there.
export function SearchBox(props: TextInputProps) {
  const [params] = useSearchParams();
  const navigate = useNavigate();
  const { pathname } = useLocation();
  const onHome = pathname === "/";
  const value = onHome ? (params.get("q") ?? "") : "";

  const search = (q: string) => navigate(q ? `/?q=${encodeURIComponent(q)}` : "/", { replace: onHome });

  return (
    <TextInput
      placeholder="Search your library"
      aria-label="Search your library"
      leftSection={<SearchIcon />}
      rightSection={value ? <CloseButton size="sm" aria-label="Clear search" onClick={() => search("")} /> : null}
      value={value}
      onChange={(e) => search(e.currentTarget.value)}
      onKeyDown={(e) => e.key === "Escape" && search("")}
      radius="xl"
      {...props}
    />
  );
}

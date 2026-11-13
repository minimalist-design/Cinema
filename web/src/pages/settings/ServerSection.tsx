import { Code, Table } from "@mantine/core";
import type { Settings } from "../../api";
import { SettingsCard } from "./common";

export function ServerSection({ settings }: { settings: Settings }) {
  const rows = [
    { label: "Port", env: "PORT", value: settings.server.port },
    { label: "Database", env: "DB_PATH", value: settings.server.dbPath },
    { label: "Cache folder", env: "CACHE_DIR", value: settings.server.cacheDir },
  ];
  return (
    <SettingsCard
      title="Server"
      description="These are read from .env (or environment variables) when Cinema starts. To change one, edit .env and restart."
    >
      <Table variant="vertical" layout="fixed" withTableBorder>
        <Table.Tbody>
          {rows.map((r) => (
            <Table.Tr key={r.env}>
              <Table.Th w={160}>{r.label}</Table.Th>
              <Table.Td style={{ wordBreak: "break-all" }}>
                <Code>{r.value}</Code>
              </Table.Td>
              <Table.Td w={120} c="dimmed" fz="xs" visibleFrom="xs">
                {r.env}
              </Table.Td>
            </Table.Tr>
          ))}
        </Table.Tbody>
      </Table>
    </SettingsCard>
  );
}

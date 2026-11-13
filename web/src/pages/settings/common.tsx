import { useState } from "react";
import type { ReactNode } from "react";
import { Box, Group, Paper, Text, Title } from "@mantine/core";
import { notifications } from "@mantine/notifications";
import { api, type Settings, type SettingsPatch } from "../../api";

// SettingsCard is one group of settings, with its actions in the footer.
export function SettingsCard({
  title,
  description,
  badge,
  footer,
  children,
}: {
  title: string;
  description?: ReactNode;
  badge?: ReactNode;
  footer?: ReactNode;
  children?: ReactNode;
}) {
  return (
    <Paper withBorder p="lg">
      <Group justify="space-between" align="flex-start" wrap="nowrap" gap="sm">
        <Box miw={0}>
          <Title order={2} fz="h4">
            {title}
          </Title>
          {description ? (
            <Text size="sm" c="dimmed" mt={4}>
              {description}
            </Text>
          ) : null}
        </Box>
        {badge}
      </Group>
      {children ? <Box mt="md">{children}</Box> : null}
      {footer ? (
        <Group justify="flex-end" gap="sm" mt="lg">
          {footer}
        </Group>
      ) : null}
    </Paper>
  );
}

// useSaveSettings saves a patch, reporting the result as a notification.
export function useSaveSettings(onSaved: (s: Settings) => void) {
  const [saving, setSaving] = useState(false);
  const save = async (patch: SettingsPatch, message: string) => {
    setSaving(true);
    try {
      onSaved(await api.updateSettings(patch));
      notifications.show({ color: "teal", title: "Saved", message });
      return true;
    } catch (e) {
      notifications.show({ color: "red", title: "Not saved", message: (e as Error).message, autoClose: 8000 });
      return false;
    } finally {
      setSaving(false);
    }
  };
  return { saving, save };
}

export const plural = (n: number, one: string, many: string) => `${n} ${n === 1 ? one : many}`;

const languageNames = (() => {
  try {
    return new Intl.DisplayNames(undefined, { type: "language" });
  } catch {
    return null;
  }
})();

// languageName names a language code ("fr" → "French"), or returns the code.
export function languageName(code: string) {
  try {
    const name = languageNames?.of(code);
    return name && name !== code ? name : code;
  } catch {
    return code;
  }
}

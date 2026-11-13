import { useState } from "react";
import { Anchor, Badge, Button, PasswordInput, Select, Stack, Text } from "@mantine/core";
import type { Settings } from "../../api";
import { SettingsCard, languageName, useSaveSettings } from "./common";

// TMDB metadata languages (ISO 639-1 + country, as TMDB uses them).
const locales = [
  "en-US", "en-GB", "en-AU", "en-CA", "es-ES", "es-MX", "fr-FR", "fr-CA", "de-DE", "it-IT", "pt-BR", "pt-PT",
  "nl-NL", "sv-SE", "da-DK", "nb-NO", "fi-FI", "pl-PL", "cs-CZ", "hu-HU", "ro-RO", "el-GR", "tr-TR", "ru-RU",
  "uk-UA", "he-IL", "ar-SA", "hi-IN", "th-TH", "vi-VN", "id-ID", "ja-JP", "ko-KR", "zh-CN", "zh-TW",
];

const localeLabel = (code: string) => {
  const name = languageName(code);
  return name !== code ? `${name} (${code})` : code;
};

function TokenCard({ settings, onSaved }: { settings: Settings; onSaved: (s: Settings) => void }) {
  const [token, setToken] = useState("");
  const { saving, save } = useSaveSettings(onSaved);

  const badge = !settings.tmdbTokenSet ? (
    <Badge color="red" variant="light">
      Not set
    </Badge>
  ) : settings.tmdbTokenSource === "settings" ? (
    <Badge color="teal" variant="light">
      Set
    </Badge>
  ) : (
    <Badge color="blue" variant="light">
      Set in .env
    </Badge>
  );

  return (
    <SettingsCard
      title="TMDB access token"
      badge={badge}
      description={
        <>
          Cinema gets posters, descriptions and episode lists from The Movie Database. Create a free account and copy the{" "}
          <b>API Read Access Token</b> from{" "}
          <Anchor href="https://www.themoviedb.org/settings/api" target="_blank" rel="noreferrer" inherit>
            themoviedb.org/settings/api
          </Anchor>
          .
        </>
      }
      footer={
        <>
          {settings.tmdbTokenSource === "settings" ? (
            <Button variant="subtle" color="red" disabled={saving} onClick={() => save({ tmdbToken: "" }, "The saved token was removed.")}>
              Remove saved token
            </Button>
          ) : null}
          <Button
            loading={saving}
            disabled={!token.trim()}
            onClick={async () => {
              if (await save({ tmdbToken: token }, "TMDB accepted the token.")) setToken("");
            }}
          >
            {settings.tmdbTokenSet ? "Replace token" : "Save token"}
          </Button>
        </>
      }
    >
      <PasswordInput
        label={settings.tmdbTokenSet ? "New token" : "Token"}
        description="It's checked with TMDB before it's saved."
        placeholder="eyJhbGciOiJIUzI1NiJ9…"
        value={token}
        onChange={(e) => setToken(e.currentTarget.value)}
        autoComplete="off"
      />
      {settings.tmdbTokenSource === "settings" ? (
        <Text size="xs" c="dimmed" mt="xs">
          This token overrides TMDB_TOKEN in .env. Remove it to use that one again.
        </Text>
      ) : null}
    </SettingsCard>
  );
}

function LanguageCard({ settings, onSaved }: { settings: Settings; onSaved: (s: Settings) => void }) {
  const [lang, setLang] = useState(settings.tmdbLanguage);
  const { saving, save } = useSaveSettings(onSaved);
  const codes = locales.includes(settings.tmdbLanguage) ? locales : [settings.tmdbLanguage, ...locales];
  const data = codes.map((c) => ({
    value: c,
    label: c === settings.defaults.tmdbLanguage ? `${localeLabel(c)} · default` : localeLabel(c),
  }));

  return (
    <SettingsCard
      title="Metadata language"
      description="The language of titles, descriptions and artwork. Titles fetched from now on use it; to update the ones already in your library, use Refresh all metadata under Library."
      footer={
        <Button
          loading={saving}
          disabled={lang === settings.tmdbLanguage}
          onClick={() => save({ tmdbLanguage: lang }, "New titles will use this language.")}
        >
          Save
        </Button>
      }
    >
      <Select
        label="Language"
        data={data}
        value={lang}
        onChange={(v) => v && setLang(v)}
        searchable
        allowDeselect={false}
        nothingFoundMessage="No such language"
        maw={360}
      />
    </SettingsCard>
  );
}

export function MetadataSection({ settings, onSaved }: { settings: Settings; onSaved: (s: Settings) => void }) {
  return (
    <Stack gap="lg">
      <TokenCard settings={settings} onSaved={onSaved} />
      <LanguageCard settings={settings} onSaved={onSaved} />
    </Stack>
  );
}

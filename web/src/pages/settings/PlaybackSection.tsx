import { useState } from "react";
import { Alert, Badge, Button, Code, Group, Stack, TagsInput, Text, TextInput } from "@mantine/core";
import type { Settings } from "../../api";
import { InfoIcon } from "../../components/icons";
import { SettingsCard, languageName, useSaveSettings } from "./common";

const suggestions = [
  "en", "es", "fr", "de", "it", "pt", "nl", "sv", "da", "no", "fi", "pl", "cs", "hu", "ro", "el", "tr", "ru",
  "uk", "he", "ar", "hi", "th", "vi", "id", "ja", "ko", "zh",
];

function SubtitlesCard({ settings, onSaved }: { settings: Settings; onSaved: (s: Settings) => void }) {
  const [langs, setLangs] = useState(settings.subtitleLanguages);
  const { saving, save } = useSaveSettings(onSaved);
  const dirty = langs.join(",") !== settings.subtitleLanguages.join(",");

  return (
    <SettingsCard
      title="Subtitles"
      description="Subtitles are always in the player's captions menu. These languages decide which one is turned on when a video starts."
      footer={
        <Button loading={saving} disabled={!dirty} onClick={() => save({ subtitleLanguages: langs }, "Applies to the next video you play.")}>
          Save
        </Button>
      }
    >
      <TagsInput
        label="Preferred languages, in order"
        description="Language codes such as en, fr or pt-BR. Leave empty to show the track the file marks as default."
        placeholder={langs.length ? undefined : "Add a language"}
        data={suggestions}
        value={langs}
        onChange={setLangs}
        renderOption={({ option }) => (
          <Group gap="xs">
            <Text size="sm">{languageName(option.value)}</Text>
            <Text size="xs" c="dimmed">
              {option.value}
            </Text>
          </Group>
        )}
        clearable
        splitChars={[",", " "]}
        maw={480}
      />
      <Text size="sm" c="dimmed" mt="xs">
        {langs.length
          ? `Turns on ${langs.map(languageName).join(", then ")} subtitles, whichever the video has first.`
          : "Turns on the subtitles the file marks as default, if any."}
      </Text>
    </SettingsCard>
  );
}

function TranscodingCard({ settings, onSaved }: { settings: Settings; onSaved: (s: Settings) => void }) {
  const [ffmpeg, setFFmpeg] = useState(settings.ffmpegPath);
  const [ffprobe, setFFprobe] = useState(settings.ffprobePath);
  const { saving, save } = useSaveSettings(onSaved);
  const dirty = ffmpeg !== settings.ffmpegPath || ffprobe !== settings.ffprobePath;
  const { media, defaults } = settings;

  return (
    <SettingsCard
      title="Transcoding"
      badge={
        <Badge color={media.available ? "teal" : "red"} variant="light">
          {media.available ? "Available" : "Unavailable"}
        </Badge>
      }
      description="ffmpeg converts MKV files for the browser as they play, and extracts their built-in subtitles. Without it, files are served as they are."
      footer={
        <Button
          loading={saving}
          disabled={!dirty}
          onClick={() => save({ ffmpegPath: ffmpeg, ffprobePath: ffprobe }, "Restart Cinema to use the new paths.")}
        >
          Save
        </Button>
      }
    >
      <Stack gap="sm">
        {!media.available && media.error ? (
          <Alert color="red" variant="light" title="ffmpeg couldn't be started">
            <Code block>{media.error}</Code>
          </Alert>
        ) : null}
        <Group grow align="flex-start">
          <TextInput
            label="ffmpeg"
            description="A command on the PATH, or a full path"
            placeholder={defaults.ffmpegPath}
            value={ffmpeg}
            onChange={(e) => setFFmpeg(e.currentTarget.value)}
            ff="monospace"
          />
          <TextInput
            label="ffprobe"
            description="A command on the PATH, or a full path"
            placeholder={defaults.ffprobePath}
            value={ffprobe}
            onChange={(e) => setFFprobe(e.currentTarget.value)}
            ff="monospace"
          />
        </Group>
        {media.restartRequired ? (
          <Alert color="yellow" variant="light" icon={<InfoIcon />}>
            Restart Cinema to use the saved paths. Until then it keeps using the ones it started with.
          </Alert>
        ) : null}
      </Stack>
    </SettingsCard>
  );
}

export function PlaybackSection({ settings, onSaved }: { settings: Settings; onSaved: (s: Settings) => void }) {
  return (
    <Stack gap="lg">
      <SubtitlesCard settings={settings} onSaved={onSaved} />
      <TranscodingCard settings={settings} onSaved={onSaved} />
    </Stack>
  );
}

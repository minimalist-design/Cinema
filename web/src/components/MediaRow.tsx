import { Link } from "react-router-dom";
import { Anchor, Box, Group, Scroller, Title } from "@mantine/core";
import { PosterCard, type MediaItem } from "./PosterCard";
import classes from "./MediaRow.module.css";

// MediaRow is a titled, horizontally scrolling row of posters, with a link
// to the full list.
export function MediaRow({
  title,
  items,
  seeAll,
  withKind = false,
}: {
  title: string;
  items: MediaItem[];
  seeAll?: string;
  withKind?: boolean;
}) {
  if (items.length === 0) return null;
  return (
    <Box component="section">
      <Group justify="space-between" align="baseline" mb="sm">
        <Title order={2} fz="h3">
          {title}
        </Title>
        {seeAll ? (
          <Anchor component={Link} to={seeAll} size="sm" fw={500}>
            See all
          </Anchor>
        ) : null}
      </Group>
      <Scroller scrollAmount={600} className={classes.scroller}>
        <div className={classes.track}>
          {items.map((item) => (
            <div key={`${item.kind}-${item.id}`} className={classes.item}>
              <PosterCard item={item} withKind={withKind} />
            </div>
          ))}
        </div>
      </Scroller>
    </Box>
  );
}

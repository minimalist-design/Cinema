import { AspectRatio, Center, Image } from "@mantine/core";
import { FilmIcon } from "./icons";
import classes from "./Artwork.module.css";

// Artwork is a poster or still at a fixed aspect ratio, with a placeholder
// when there's no image.
export function Artwork({ src, ratio, lazy = true }: { src: string | null; ratio: number; lazy?: boolean }) {
  return (
    <AspectRatio ratio={ratio} className={classes.frame}>
      {src ? (
        <Image src={src} alt="" loading={lazy ? "lazy" : undefined} className={classes.image} />
      ) : (
        <Center className={classes.placeholder}>
          <FilmIcon />
        </Center>
      )}
    </AspectRatio>
  );
}

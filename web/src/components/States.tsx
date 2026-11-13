import { Alert, Center, EmptyState, Loader } from "@mantine/core";
import type { ReactNode } from "react";
import { AlertIcon } from "./icons";

export function PageLoader() {
  return (
    <Center py={80}>
      <Loader />
    </Center>
  );
}

export function ErrorAlert({ children }: { children: ReactNode }) {
  return (
    <Alert color="red" variant="light" icon={<AlertIcon />} title="Something went wrong" role="alert">
      {children}
    </Alert>
  );
}

export function Message({ title, description }: { title: ReactNode; description?: ReactNode }) {
  return <EmptyState py="xl" title={title} description={description} />;
}

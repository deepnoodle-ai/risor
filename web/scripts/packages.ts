// The npm packages, in the order they publish: each one after the packages
// it depends on.

export const PACKAGES = [
  { name: "@deepnoodle/risor", dir: "../ts" },
  { name: "@deepnoodle/risor-service", dir: "packages/service" },
  { name: "@deepnoodle/risor-codemirror", dir: "packages/codemirror" },
  { name: "@deepnoodle/risor-react", dir: "packages/react" },
  { name: "@deepnoodle/risor-mantine", dir: "packages/mantine" },
] as const;

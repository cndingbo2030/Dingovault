# @cndingbo2030/dingovault-sdk

Published to **GitHub Packages** (npm registry) on each version tag. This package is a **stub** today: it reserves the name and registry wiring so future releases can ship typed helpers, event schemas, or CLI tooling for plugin authors.

## Install

Configure npm for the GitHub scope (one-time), then install:

```bash
echo "@cndingbo2030:registry=https://npm.pkg.github.com" >> ~/.npmrc
# Use a GitHub PAT with read:packages if the repo/package is private
npm install @cndingbo2030/dingovault-sdk
```

Public repositories still require authentication to install from `npm.pkg.github.com` unless you use a fine-grained token; see [GitHub Docs — Working with the npm registry](https://docs.github.com/packages/working-with-a-github-packages-registry/working-with-the-npm-registry).

## License

AGPL-3.0 — same as [Dingovault](https://github.com/cndingbo2030/dingovault).

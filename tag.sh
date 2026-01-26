#!/bin/bash

set -e

VERSION=$1

if [ -z "$VERSION" ]; then
    echo "Usage: tag.sh <version>"
    exit 1
fi

git tag $VERSION
git tag cmd/risor/$VERSION
git tag modules/ssh/$VERSION

git push origin $VERSION
git push origin cmd/risor/$VERSION
git push origin modules/ssh/$VERSION

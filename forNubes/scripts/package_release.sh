#!/bin/bash
set -e

# Cleanup
rm -rf build_artifacts
mkdir -p build_artifacts

VERSION="${1:-2.0.0}"

# Function to build and zip
build_and_zip() {
    OS=$1
    ARCH=$2
    echo "Building for $OS/$ARCH..."
    
    BINARY_NAME="terraform-provider-nubes_v${VERSION}"
    if [ "$OS" == "windows" ]; then
        BINARY_NAME="${BINARY_NAME}.exe"
    fi
    
    env CGO_ENABLED=0 GOOS=$OS GOARCH=$ARCH go build -o build_artifacts/${BINARY_NAME} .
    
    cd build_artifacts
    ZIP_NAME="terraform-provider-nubes_${VERSION}_${OS}_${ARCH}.zip"
    
    # Zip the binary
    if [[ "$OS" == "windows" ]]; then
       # For windows, we might need zip to handle the .exe extension properly if we were relying on unix permissions, but zip handles it ok.
       # Using python zipfile as 'zip' command was missing earlier
       python3 -m zipfile -c $ZIP_NAME $BINARY_NAME
    else
       # Ensure executable permission
       chmod +x $BINARY_NAME
       python3 -m zipfile -c $ZIP_NAME $BINARY_NAME
    fi
    
    # Remove binary to save space/confusion (optional, but cleaner)
    rm $BINARY_NAME
    cd ..
}

# Build for all targets
build_and_zip linux amd64
build_and_zip windows amd64
build_and_zip darwin amd64
build_and_zip darwin arm64

echo "Calculating SHA256SUMS..."
cd build_artifacts
sha256sum *.zip > terraform-provider-nubes_${VERSION}_SHA256SUMS

echo "Signing SHA256SUMS..."
# Detached binary signature
gpg --batch --detach-sign --default-key 866FD93D456DCA800F2448413EC4673EB798238A --output terraform-provider-nubes_${VERSION}_SHA256SUMS.sig terraform-provider-nubes_${VERSION}_SHA256SUMS

echo "Uploading to S3..."
# Base S3 path
S3_PATH="registry/terraform-registry/terra.k8c.ru/nubes/nubes/${VERSION}/"

# Upload zips
for f in *.zip; do
    mc cp $f $S3_PATH
done

# Upload sums and sig
mc cp terraform-provider-nubes_${VERSION}_SHA256SUMS $S3_PATH
mc cp terraform-provider-nubes_${VERSION}_SHA256SUMS.sig $S3_PATH

echo "Done!"

#!/usr/bin/env bash
# Copyright (c) HashiCorp, Inc.
# SPDX-License-Identifier: MPL-2.0

set -euo pipefail

componentTypeFromFolderName() {
    if [[ "$1" = "builders" ]]; then
        echo "builder"
    elif [[ "$1" = "provisioners" ]]; then
        echo "provisioner"
    elif [[ "$1" = "post-processors" ]]; then
        echo "post-processor"
    elif [[ "$1" = "datasources" ]]; then
        echo "data-source"
    else
        echo ""
    fi
}

rewriteLinks() {
  local result="$1"
  local organization="$2"

  local urlSegment="([^/]+)"
  local urlAnchor="(#[^/]+)"

  local find="\(\/packer\/plugins\/$urlSegment\/$urlSegment$urlAnchor?\)"
  local replace="\(\/packer\/integrations\/$organization\/\2\3\)"
  result="$(echo "$result" | sed -E "s/$find/$replace/g")"

  find="\(\/packer\/plugins\/$urlSegment\/$urlSegment\/$urlSegment$urlAnchor?\)"
  replace="\(\/packer\/integrations\/$organization\/\2\/latest\/components\/\1\/\3\4\)"
  result="$(echo "$result" | sed -E "s/$find/$replace/g")"

  result="$(echo "$result" \
      | sed "s/\/datasources\//\/data-source\//g" \
      | sed "s/\/builders\//\/builder\//g" \
      | sed "s/\/post-processors\//\/post-processor\//g" \
      | sed "s/\/provisioners\//\/provisioner\//g" \
  )"

  echo "$result"
}

processComponentFile() {
    local docsDir="$1"
    local webDocsDir="$2"
    local componentFile="$3"
    local organization="$4"

    local escapedDocsDir
    escapedDocsDir="$(echo "$docsDir" | sed 's/\//\\\//g' | sed 's/\./\\\./g')"
    local componentTypeAndSlug
    componentTypeAndSlug="$(echo "$componentFile" | sed "s/$escapedDocsDir\///g" | sed 's/\.mdx//g')"

    local componentSlug
    componentSlug="$(echo "$componentTypeAndSlug" | cut -d'/' -f 2)"
    local componentType
    componentType="$(componentTypeFromFolderName "$(echo "$componentTypeAndSlug" | cut -d'/' -f 1)")"
    if [[ "$componentType" = "" ]]; then
        echo "Failed to process '$componentFile', unexpected folder name."
        echo "Documentation for components must be stored in one of:"
        echo "builders, provisioners, post-processors, datasources"
        exit 1
    fi

    local webDocsFolder="$webDocsDir/components/$componentType/$componentSlug"
    mkdir -p "$webDocsFolder"
    local webDocsFile="$webDocsFolder/README.md"
    local webDocsFileTmp="$webDocsFolder/README.md.tmp"

    cp "$componentFile" "$webDocsFile"

    local lastMetadataLine
    lastMetadataLine="$(grep -n -m 2 '^\-\-\-' "$componentFile" | tail -n1 | cut -d':' -f1)"
    tail -n +"$((lastMetadataLine+2))" "$webDocsFile" > "$webDocsFileTmp"
    mv "$webDocsFileTmp" "$webDocsFile"

    tail -n +3 "$webDocsFile" > "$webDocsFileTmp"
    mv "$webDocsFileTmp" "$webDocsFile"

    rewriteLinks "$(cat "$webDocsFile")" "$organization" > "$webDocsFileTmp"
    mv "$webDocsFileTmp" "$webDocsFile"
}

compileWebDocs() {
  local docsDir="$1/$2"
  local webDocsDir="$1/$3"
  local organization="$4"

  echo "Compiling MDX docs in '$2' to Markdown in '$3'..."
  mkdir -p "$webDocsDir"

  cp "$docsDir/README.md" "$webDocsDir/README.md"

  while IFS= read -r file; do
    processComponentFile "$docsDir" "$webDocsDir" "$file" "$organization"
  done < <(find "$docsDir" -path "$docsDir/*/*.mdx" ! -name "index.mdx" | sort)
}

compileWebDocs "$1" "$2" "$3" "$4"

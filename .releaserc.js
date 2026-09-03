const fs = require('fs');
const path = require('path');

const headerPartial = fs.readFileSync(
  path.resolve('.github/workflows/assets/release-notes-header.hbs'),
  'utf8'
);

module.exports = {
  branches: ['main'],
  plugins: [
    '@semantic-release/commit-analyzer',
    [
      '@semantic-release/release-notes-generator',
      {
        writerOpts: {
          headerPartial: headerPartial
        }
      }
    ],
    [
      '@semantic-release/npm',
      {
        npmPublish: false
      }
    ],
    [
      '@semantic-release/exec',
      {
        prepareCmd: '.github/workflows/assets/release.sh ${nextRelease.version}'
      }
    ],
    [
      '@semantic-release/git',
      {
        assets: ['package.json', 'package-lock.json'],
        message:
          'refactor(release): bump version to ${nextRelease.version} [Actions Bot]'
      }
    ],
    '@semantic-release/github'
  ]
};

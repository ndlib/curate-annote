import Mirador from 'mirador/dist/es/src/index';
import { miradorImageToolsPlugin } from 'mirador-image-tools';

const config = {
  id: 'demo',
  windows: [{
    imageToolsEnabled: true,
    manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/durer',
  }],
  theme: {
    palette: {
      primary: {
        main: '#1967d2',
      },
    },
  },
};

Mirador.viewer(config, [
  ...miradorImageToolsPlugin,
]);

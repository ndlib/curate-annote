import mirador from 'mirador/dist/es/src/index';
import annotationPlugins from 'mirador-annotations';
import AnnototAdapter from 'mirador-annotations/es/AnnototAdapter';
import LocalStorageAdapter from 'mirador-annotations/es/LocalStorageAdapter';

const config = {
  id: 'demo',
  annotation: {
    adapter: (canvasId) => new LocalStorageAdapter(`localStorage://?canvasId=${canvasId}`),
    // adapter: (canvasId) => new AnnototAdapter(canvasId, 'https://annotation-test.library.nd.edu/annotations'),
  },
  window: {
    defaultSideBarPanel: 'annotations',
    sideBarOpenByDefault: true,
  },
  windows: [{
    manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/durer',
  }],
};

mirador.viewer(config, [
  ...annotationPlugins,
]);

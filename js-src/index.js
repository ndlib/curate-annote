import mirador from 'mirador/dist/es/src/index';
import annotationPlugins from 'mirador-annotations';
import AnnototAdapter from 'mirador-annotations/es/AnnototAdapter';
import LocalStorageAdapter from 'mirador-annotations/es/LocalStorageAdapter';

const config = {
  id: 'demo',
  annotation: {
    // adapter: (canvasId) => new LocalStorageAdapter(`localStorage://?canvasId=${canvasId}`),
      adapter: (canvasId) => new AnnototAdapter(canvasId, 'http://localhost:8888/annotot'),
  },
  window: {
    defaultSideBarPanel: 'annotations',
    sideBarOpenByDefault: true,
  },
  windows: [{
    manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/durer',
  }],
  catalog: [{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/harriot_6782',
      provider: 'Hesburgh Libraries'
  },{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/harriot_6783',
      provider: 'Hesburgh Libraries'
  },{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/harriot_6784',
      provider: 'Hesburgh Libraries'
  },{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/harriot_6785',
      provider: 'Hesburgh Libraries'
  },{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/harriot_6786',
      provider: 'Hesburgh Libraries'
  },{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/New_retrotechAZ',
      provider: 'Hesburgh Libraries'
  },{
      manifestId: 'https://iiif-cds.library.nd.edu/iiif/manifest/CampoMarzo',
      provider: 'Hesburgh Libraries'
  }]
};

mirador.viewer(config, [
  ...annotationPlugins,
]);

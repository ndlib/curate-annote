const path = require('path');

module.exports = {
  entry: './js-src/index.js',
  module: {
   rules: [
     {
       test: /\.css$/,
       use: [
         'style-loader',
         'css-loader',
       ],
     },
   ],
  },
  output: {
    filename: 'mirador.js',
    path: path.resolve(__dirname, '../web/static'),
    publicPath: '/static/',
  },
};

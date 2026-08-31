const defaultConfig = require('@wordpress/prettier-config');
const config = {
	...defaultConfig,
	overrides: [
		...(defaultConfig.overrides ?? []),
		{
			files: '*.yml',
			options: {
				tabWidth: 2,
				singleQuote: false,
			},
		},
		{
			files: '*',
			options: {
				printWidth: 100,
			},
		},
	],
	plugins: [...(defaultConfig.plugins ?? []), '@zackad/prettier-plugin-twig'],
};
module.exports = config;

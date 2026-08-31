#!/usr/bin/env node
import * as esbuild from 'esbuild';
import { sassPlugin } from 'esbuild-sass-plugin';
import postcss from 'postcss';
import postcssPresetEnv from 'postcss-preset-env';
import { readdirSync, readFileSync, existsSync } from 'node:fs';
import { join, parse } from 'node:path';
import { parse as tomlParse } from 'smol-toml';

const config = {
	parent: 'jcore2',
	theme: 'jcore2-child',
};
if (existsSync('defaults.toml')) {
	Object.assign(config, tomlParse(readFileSync('defaults.toml', 'utf8')));
}
if (existsSync('jcore.toml')) {
	Object.assign(config, tomlParse(readFileSync('jcore.toml', 'utf8')));
}
if (existsSync('.localConfig.toml')) {
	Object.assign(config, tomlParse(readFileSync('.localConfig.toml', 'utf8')));
}

const jcorePath = join('wp-content/themes', config.parent);
const childPath = join('wp-content/themes', config.theme);

const buildData = {
	scripts: [
		{
			entryPoints: [join(childPath, 'js'), join(jcorePath, 'js')],
			outdir: join(childPath, '/dist/js'),
		},
	],
	styles: [
		{
			entryPoints: [join(childPath, 'scss'), join(jcorePath, 'scss')],
			outdir: join(childPath, '/dist/css'),
		},
	],
};
if (existsSync('build.toml')) {
	Object.assign(buildData, tomlParse(readFileSync('build.toml', 'utf8')));
}

const options = {
	bundle: true,
	minify: true,
	write: true,
	sourcemap: true,
	logLevel: 'info',
	entryNames: '[name]',
	external: ['*.png', '*.svg', '*.jpg', '*.jpeg', '*.css', '*.woff', '*.woff2', '*.otf'],
};

const sassOptions = {
	async transform(source, resolveDir) {
		const { css } = await postcss([postcssPresetEnv({ stage: 2 })]).process(source, {
			from: join(childPath, 'dist/css'),
		});
		return css;
	},
};

// Scripts
for (const script of buildData.scripts) {
	const entries = [];
	for (const entry of script.entryPoints) {
		entries.push(...getFiles(entry));
	}
	const config = {
		...options,
		entryPoints: deDupe(entries),
		loader: { '.js': 'jsx' },
		outdir: script.outdir,
	};
	await build(config);
}

// Styles
for (const style of buildData.styles) {
	const entries = [];
	for (const entry of style.entryPoints) {
		entries.push(...getFiles(entry));
	}
	const config = {
		...options,
		entryPoints: deDupe(entries),
		plugins: [sassPlugin(sassOptions)],
		outdir: style.outdir,
	};
	await build(config);
}

async function build(config) {
	if (process.argv[2] === 'build') {
		await esbuild.build(config);
	} else if (process.argv[2] === 'watch') {
		// ESbuild 0.17 syntax.
		await (await esbuild.context(config)).watch();
	}
}

/**
 * Get all files that needs to be passed to esbuild from the given path.
 *
 * @param path
 * @returns {*[]}
 */
function getFiles(path) {
	const files = [];
	if (existsSync(path)) {
		for (const file of readdirSync(path)) {
			// Only add files not starting with _ and that has an extension.
			if (file.substring(0, 1) !== '_' && file.split('.').length > 1) {
				files.push(join(path, file));
			}
		}
	}
	return files;
}

/**
 * Remove duplicate filenames from list to allow "overwrites".
 * @param files array<string>
 * @return array<string>
 */
function deDupe(files) {
	const names = [];
	return files.filter((file) => {
		const path = parse(file);
		if (!names.includes(path.name)) {
			names.push(path.name);
			return true;
		}
		return false;
	});
}

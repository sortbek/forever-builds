// Reads each spec's `defaults` block from the engine's spec.ts/presets.ts without running them and
// writes the parts a headless sim needs as protojson, plus a hash of each spec's rotation choice.
// Usage: node scripts/extract-defaults.mjs <engine dir> <out dir> <rotation lock file>
import crypto from 'node:crypto';
import fs from 'node:fs';
import path from 'node:path';
import ts from 'typescript';

const [engineDir, outDir, lockFile] = process.argv.slice(2);

const SPECS = {
	balanceDruid: 'druid/balance',
	feralCatDruid: 'druid/feralcat',
	feralBearDruid: 'druid/feralbear',
	hunter: 'hunter/dps',
	mage: 'mage/dps',
	protectionPaladin: 'paladin/protection',
	retributionPaladin: 'paladin/retribution',
	dpsPriest: 'priest/dps',
	rogue: 'rogue/dps',
	elementalShaman: 'shaman/elemental',
	enhancementShaman: 'shaman/enhancement',
	warlock: 'warlock/dps',
	dpsWarrior: 'warrior/dps',
	protectionWarrior: 'warrior/protection',
};
const KEYS = ['specOptions', 'consumables', 'raidBuffs', 'partyBuffs', 'individualBuffs', 'debuffs', 'other', 'talents'];
const ROTATION_KEYS = ['autoRotation', 'rotationType', 'aplRotation'];

// tsconfig path aliases ('@sim/*' -> ui/sim/*).
const ALIASES = Object.entries(
	ts.parseConfigFileTextToJson('tsconfig.json', fs.readFileSync(path.join(engineDir, 'tsconfig.json'), 'utf8')).config.compilerOptions.paths,
).map(([from, [to]]) => [from.slice(0, -1), path.resolve(engineDir, to.slice(0, -1))]);

function importFile(sf, spec) {
	let base;
	if (spec.startsWith('.')) base = path.resolve(path.dirname(sf.fileName), spec);
	else {
		const alias = ALIASES.find(([from]) => spec.startsWith(from));
		if (!alias) return null;
		base = path.join(alias[1], spec.slice(alias[0].length));
	}
	const file = /\.(ts|json)$/.test(base) ? base : `${base}.ts`;
	return fs.existsSync(file) ? file : null;
}

const files = new Map();
function source(file) {
	if (!files.has(file)) {
		files.set(file, ts.createSourceFile(file, fs.readFileSync(file, 'utf8'), ts.ScriptTarget.Latest, true));
	}
	return files.get(file);
}

function fail(sf, node, msg) {
	const { line } = sf.getLineAndCharacterOfPosition(node.getStart(sf));
	throw new Error(`${path.relative(engineDir, sf.fileName)}:${line + 1}: ${msg}: ${node.getText(sf).slice(0, 80)}`);
}

// Relative import bindings: local name -> { file, exported name | '*' }.
function imports(sf) {
	const out = new Map();
	for (const st of sf.statements) {
		if (!ts.isImportDeclaration(st) || !st.importClause) continue;
		const spec = st.moduleSpecifier.text;
		const file = importFile(sf, spec);
		if (!file) continue;
		if (st.importClause.name && file.endsWith('.json')) out.set(st.importClause.name.text, { file, name: 'json' });
		if (file.endsWith('.json')) continue;
		const nb = st.importClause.namedBindings;
		if (nb && ts.isNamespaceImport(nb)) out.set(nb.name.text, { file, name: '*' });
		if (nb && ts.isNamedImports(nb)) {
			for (const el of nb.elements) out.set(el.name.text, { file, name: (el.propertyName ?? el.name).text });
		}
	}
	return out;
}

function topLevelConst(sf, name) {
	for (const st of sf.statements) {
		if (!ts.isVariableStatement(st)) continue;
		for (const d of st.declarationList.declarations) {
			if (ts.isIdentifier(d.name) && d.name.text === name) return d.initializer;
		}
	}
	return null;
}

function resolveName(sf, name) {
	const init = topLevelConst(sf, name);
	if (init) return [sf, init];
	const imp = imports(sf).get(name);
	if (!imp || imp.name === '*') return null;
	return resolveName(source(imp.file), imp.name);
}

// Argument values of the local helper calls being evaluated, innermost last.
const scopes = [];

function evaluate(sf, node) {
	if (ts.isParenthesizedExpression(node) || ts.isAsExpression(node) || ts.isSatisfiesExpression(node)) return evaluate(sf, node.expression);
	if (ts.isNumericLiteral(node)) return Number(node.text);
	if (ts.isPrefixUnaryExpression(node) && node.operator === ts.SyntaxKind.MinusToken) return -evaluate(sf, node.operand);
	if (ts.isStringLiteral(node)) return node.text;
	if (node.kind === ts.SyntaxKind.TrueKeyword) return true;
	if (node.kind === ts.SyntaxKind.FalseKeyword) return false;
	if (ts.isArrayLiteralExpression(node)) return node.elements.map(e => evaluate(sf, e));
	if (ts.isObjectLiteralExpression(node)) {
		const out = {};
		for (const p of node.properties) {
			if (ts.isPropertyAssignment(p)) out[p.name.getText(sf)] = evaluate(sf, p.initializer);
			else if (ts.isShorthandPropertyAssignment(p)) out[p.name.text] = evaluate(sf, p.name);
			else if (ts.isSpreadAssignment(p)) Object.assign(out, evaluate(sf, p.expression));
			else fail(sf, p, 'unsupported property');
		}
		return out;
	}
	if (ts.isCallExpression(node) && ts.isPropertyAccessExpression(node.expression)) {
		const fn = node.expression.name.text;
		// PresetUtils.makePresetTalents(name, SavedTalents.create({...})) -> { data }
		if (fn === 'makePresetTalents') return { data: evaluate(sf, node.arguments[1]) };
		// Message.create({...}) / Message.create()
		if (fn === 'create') return node.arguments.length ? evaluate(sf, node.arguments[0]) : {};
	}
	// helper(args) where helper is a top-level arrow function with an expression body.
	if (ts.isCallExpression(node) && ts.isIdentifier(node.expression)) {
		const hit = resolveName(sf, node.expression.text);
		if (hit && ts.isArrowFunction(hit[1]) && !ts.isBlock(hit[1].body)) {
			const args = node.arguments.map(a => evaluate(sf, a));
			scopes.push(new Map(hit[1].parameters.map((p, i) => [p.name.getText(hit[0]), args[i]])));
			try {
				return evaluate(hit[0], hit[1].body);
			} finally {
				scopes.pop();
			}
		}
	}
	if (ts.isIdentifier(node)) {
		const scope = scopes.at(-1);
		if (scope?.has(node.text)) return scope.get(node.text);
		const json = imports(sf).get(node.text);
		if (json?.name === 'json') return JSON.parse(fs.readFileSync(json.file, 'utf8'));
		const hit = resolveName(sf, node.text);
		if (!hit) fail(sf, node, 'unresolved identifier');
		return evaluate(hit[0], hit[1]);
	}
	if (ts.isPropertyAccessExpression(node) && ts.isIdentifier(node.expression)) {
		if (scopes.at(-1)?.has(node.expression.text)) return evaluate(sf, node.expression)[node.name.text];
		const imp = imports(sf).get(node.expression.text);
		if (imp?.name === 'json') return evaluate(sf, node.expression)[node.name.text];
		if (imp && imp.name === '*') {
			const hit = resolveName(source(imp.file), node.name.text);
			if (!hit) fail(sf, node, 'unresolved export');
			return evaluate(hit[0], hit[1]);
		}
		// Enum.Member: protojson takes the member name.
		return node.name.text;
	}
	if (ts.isPropertyAccessExpression(node)) {
		const base = evaluate(sf, node.expression);
		if (base === null || typeof base !== 'object' || !(node.name.text in base)) fail(sf, node, 'unresolved property');
		return base[node.name.text];
	}
	fail(sf, node, 'unsupported expression');
}

function specConfigProperties(sf) {
	let defaults = null;
	const rotation = [];
	const visit = n => {
		if (ts.isPropertyAssignment(n)) {
			const name = n.name.getText(sf);
			if (name === 'defaults' && ts.isObjectLiteralExpression(n.initializer) && !defaults) defaults = n.initializer;
			if (name === 'autoRotation') rotation.push(n.getText(sf));
		}
		ts.forEachChild(n, visit);
	};
	visit(sf);
	if (!defaults) throw new Error(`${sf.fileName}: no defaults block`);
	return { defaults, rotation };
}

fs.mkdirSync(outDir, { recursive: true });
const lock = {};
for (const [key, dir] of Object.entries(SPECS)) {
	const sf = source(path.join(engineDir, 'ui/specs', dir, 'spec.ts'));
	const { defaults, rotation } = specConfigProperties(sf);
	const out = {};
	for (const p of defaults.properties) {
		const name = p.name?.getText(sf);
		if (ROTATION_KEYS.includes(name)) rotation.push(p.getText(sf));
		if (KEYS.includes(name)) out[name] = evaluate(sf, p.initializer);
	}
	for (const k of KEYS) if (!(k in out)) out[k] = {};
	out.talents = out.talents.talentsString ?? '';
	const other = out.other;
	out.other = {
		reactionTimeMs: other.reactionTime || 100,
		distanceFromTarget: other.distanceFromTarget || 0,
		channelClipDelayMs: other.channelClipDelay || 0,
	};
	fs.writeFileSync(path.join(outDir, `${key}.json`), JSON.stringify(out, null, '\t') + '\n');
	lock[key] = crypto.createHash('sha256').update(rotation.join('\n')).digest('hex');
}
fs.writeFileSync(lockFile, JSON.stringify(lock, null, '\t') + '\n');
console.log(`defaults for ${Object.keys(SPECS).length} specs written to ${outDir}`);

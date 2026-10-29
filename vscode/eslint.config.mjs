import js from '@eslint/js';
import tseslint from '@typescript-eslint/eslint-plugin';

export default [
	{
		ignores: ['node_modules/**', 'client/node_modules/**', 'client/out/**'],
	},
	js.configs.recommended,
	...tseslint.configs['flat/recommended'],
	{
		rules: {
			'semi': [2, 'always'],
			'@typescript-eslint/no-unused-vars': 0,
			'@typescript-eslint/no-explicit-any': 0,
			'@typescript-eslint/explicit-module-boundary-types': 0,
			'@typescript-eslint/no-non-null-assertion': 0,
		},
	},
];

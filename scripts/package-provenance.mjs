export const packageVersionPattern = /^v\d+\.\d+\.\d+(?:-[0-9A-Za-z.-]+)?(?:\+[0-9A-Za-z.-]+)?$/

export function githubBuildSource({ GITHUB_SHA: sha, GITHUB_REPOSITORY: repository,
  GITHUB_RUN_ID: runID, GITHUB_RUN_ATTEMPT: attempt }) {
  if (!/^[0-9a-f]{40}$/.test(sha ?? '') || !/^[A-Za-z0-9_.-]+\/[A-Za-z0-9_.-]+$/.test(repository ?? '') ||
      !/^[1-9]\d*$/.test(runID ?? '') || !/^[1-9]\d*$/.test(attempt ?? '')) {
    throw new Error('Expected a GitHub repository, commit, run, and attempt')
  }
  const sourceURL = 'https://github.com/' + repository
  return '[提交 `' + sha + '`](' + sourceURL + '/commit/' + sha + ') · [打包运行 ' + runID +
    '，第 ' + attempt + ' 次](' + sourceURL + '/actions/runs/' + runID + '/attempts/' + attempt + ')'
}

export function hostedPackageHeader(env) {
  const { PACKAGE_VERSION: version, PACKAGE_MODE: mode, GITHUB_ACTIONS: inActions } = env
  if (inActions !== 'true' || !['smoke', 'release'].includes(mode) ||
      !packageVersionPattern.test(version ?? '')) {
    throw new Error('Expected provenance from a GitHub Actions package build')
  }
  return [
    '# 金帝短剧 ' + version,
    '',
    '- 打包类型 ' + (mode === 'release' ? '正式候选' : '测试包'),
    '- 构建来源 ' + githubBuildSource(env),
  ]
}

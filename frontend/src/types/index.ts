export interface Repository {
  id: string
  name: string
  path: string
}

export interface RepositoryInfo {
  overview?: RepositoryOverview | null
  lastScannedAt?: string
  lastCleanupAt?: string
}

export interface AppConfig {
  mavenPath: string
  repositories: Repository[]
  activeRepoId: string
  remoteRepositories?: RemoteRepository[]
}

export interface RemoteRepository {
  name: string
  url: string
}

export interface RepositoryOverview {
  totalFiles: number
  totalGroups: number
  totalPackages: number
  totalVersions: number
}

export interface TreeNode {
  id: string
  label: string
  isLeaf: boolean
  groupId?: string
  artifactId?: string
  children?: TreeNode[]
}

export interface TabItem {
  id: string
  title: string
  type: 'dashboard' | 'packages' | 'artifact' | 'cleanup' | 'export'
  groupId?: string
  artifactId?: string
}

export interface ArtifactCoordinate {
  groupId: string
  artifactId: string
  version: string
}

export interface ArtifactVersionSummary extends ArtifactCoordinate {
  modTime: string
}

export interface FileDetail {
  fileName: string
  size: number
  modTime: string
  path: string
  packaging?: string
}

export interface VersionGroup {
  version: string
  files: FileDetail[]
  totalSize: number
  latestModTime: string
  isPom: boolean
  hasJar: boolean
  hasSource: boolean
  hasSha1: boolean
  hasPom: boolean
  hasRemote: boolean
  hasLastUpdated: boolean
}

export interface GarbageItem {
  id: string
  path: string
  size: number
  versionSize?: number
  category: string
  reason: string
  modTime: string
  isDiagnostic?: boolean
  packageName: string
  groupId?: string
  artifactId?: string
  version?: string
}
<template>
  <div class="app-container">
    <!-- 锁屏界面 -->
    <div v-if="isLocked" class="lock-screen">
      <div class="lock-content">
        <div class="lock-icon">🔒</div>
        <h2>Google Authenticator</h2>
        <p>请输入密码解锁</p>
        <el-input
          v-model="unlockPassword"
          type="password"
          placeholder="请输入密码"
          show-password
          @keyup.enter="unlock"
          style="width: 240px; margin: 20px 0;"
        />
        <br />
        <el-button type="primary" @click="unlock">解锁</el-button>
      </div>
    </div>

    <!-- 空状态：欢迎界面 -->
    <div v-else-if="accounts.length === 0" class="empty-welcome">
      <div class="welcome-content">
        <div class="welcome-icon">🔐</div>
        <h1>Google Authenticator</h1>
        <p class="subtitle">桌面版</p>

        <div class="welcome-actions">
          <el-button type="primary" size="large" @click="addDialogVisible = true">
            📝 手动输入
          </el-button>
          <el-button size="large" @click="scanDialogVisible = true">
            📱 扫描二维码
          </el-button>
        </div>

        <p class="welcome-tip">💡 批量导入请使用菜单「文件  → 转移验证码」</p>
      </div>
    </div>

    <!-- 有账户时的主界面 -->
    <div v-else class="main-layout">
      <!-- 顶部工具栏 -->
      <div class="top-toolbar">
        <div class="toolbar-left">
          <div class="toolbar-brand">🔐 Google Authenticator</div>
          <el-button type="primary" :icon="Plus" @click="showAddDialog">添加</el-button>
        </div>
        <div class="toolbar-search">
          <el-input
            v-model="searchQuery"
            placeholder="搜索账户..."
            :prefix-icon="Search"
            clearable
            size="default"
          />
        </div>
      </div>

      <!-- 主体区域 -->
      <div class="main-body">
        <!-- 左侧分组栏 -->
        <div class="sidebar">
          <div class="sidebar-title">分组</div>
          <div
            class="group-item"
            :class="{ active: currentGroup === '' }"
            @click="currentGroup = ''"
          >
            <span class="group-icon">📁</span>
            <span class="group-name">全部</span>
            <span class="group-count">{{ accounts.length }}</span>
          </div>
          <div
            v-for="group in groups"
            :key="group"
            class="group-item"
            :class="{ active: currentGroup === group }"
            @click="currentGroup = group"
          >
            <span class="group-icon">📁</span>
            <span class="group-name">{{ group }}</span>
            <span class="group-count">{{ getGroupCount(group) }}</span>
          </div>
          <div class="group-item add-group" @click="showAddGroupDialog">
            <span class="group-icon">➕</span>
            <span class="group-name">新建分组</span>
          </div>
        </div>

        <!-- 右侧账户列表 -->
        <div class="content-area">
          <!-- 信息栏 -->
          <div class="info-bar">
            <span class="account-count">{{ filteredAccounts.length }} 个账户</span>
            <el-button text :icon="Refresh" @click="loadAccounts">刷新</el-button>
          </div>

          <!-- 账户列表 -->
          <div class="accounts-list">
            <div
              v-for="account in filteredAccounts"
              :key="account.id"
              class="account-item"
              :class="{ selected: selectedAccounts.includes(account.id) }"
              @click="toggleSelect(account.id)"
            >
              <!-- 编辑按钮 -->
              <el-icon class="edit-btn" @click.stop="openEditDialog(account)"><Edit /></el-icon>

              <div class="account-left">
                <div class="account-icon">🔵</div>
                <div class="account-info">
                  <div class="account-issuer">
                    {{ account.issuer || '未知' }}
                    <span v-if="account.group" class="account-group">· {{ account.group }}</span>
                  </div>
                  <div class="account-name">{{ account.name }}</div>
                </div>
              </div>
              <div class="account-center" @click.stop="copyCode(account)">
                <span class="code-text">{{ formatCode(codes[account.id]?.code) }}</span>
                <el-icon class="copy-icon"><CopyDocument /></el-icon>
              </div>
              <div class="account-right">
                <span class="time-text" :style="{ color: getTimeColor(codes[account.id]?.remaining) }">
                  {{ codes[account.id]?.remaining || 0 }}s
                </span>
                <el-progress
                  type="circle"
                  :percentage="100 - (codes[account.id]?.progress || 0)"
                  :width="36"
                  :stroke-width="4"
                  :color="getTimeColor(codes[account.id]?.remaining)"
                  :show-text="false"
                />
              </div>
            </div>

            <div v-if="filteredAccounts.length === 0" class="no-accounts">
              <p>{{ searchQuery ? '没有匹配的账户' : '该分组暂无账户' }}</p>
            </div>
          </div>
        </div>
      </div>

      <!-- 底部选择操作栏 -->
      <transition name="slide-up">
        <div v-if="selectedAccounts.length > 0" class="selection-bar">
          <span class="selection-info">已选中 {{ selectedAccounts.length }} 个账户</span>
          <div class="selection-actions">
            <el-dropdown trigger="click" @command="moveToGroup">
              <el-button>
                📁 移动到...<el-icon class="el-icon--right"><ArrowDown /></el-icon>
              </el-button>
              <template #dropdown>
                <el-dropdown-menu>
                  <el-dropdown-item command="">未分组</el-dropdown-item>
                  <el-dropdown-item v-for="g in groups" :key="g" :command="g">{{ g }}</el-dropdown-item>
                  <el-dropdown-item divided command="__new__">➕ 新建分组</el-dropdown-item>
                </el-dropdown-menu>
              </template>
            </el-dropdown>
            <el-button type="danger" @click="deleteSelected">🗑️ 删除</el-button>
            <el-button @click="selectedAccounts = []">取消</el-button>
          </div>
        </div>
      </transition>
    </div>

    <!-- ========== 对话框 ========== -->

    <!-- 添加方式选择 -->
    <el-dialog v-model="addChoiceVisible" title="添加账户" width="360px" align-center>
      <div class="dialog-buttons">
        <el-button size="large" @click="addDialogVisible = true; addChoiceVisible = false">
          📝 手动输入密钥
        </el-button>
        <el-button size="large" @click="scanDialogVisible = true; addChoiceVisible = false">
          📱 扫描二维码
        </el-button>
      </div>
    </el-dialog>

    <!-- 手动添加账户 -->
    <el-dialog v-model="addDialogVisible" title="添加账户" width="480px" align-center>
      <el-form label-width="80px">
        <el-form-item label="账户名" required>
          <el-input v-model="newAccount.name" placeholder="user@example.com" />
        </el-form-item>
        <el-form-item label="发行者">
          <el-input v-model="newAccount.issuer" placeholder="Google" />
        </el-form-item>
        <el-form-item label="分组">
          <el-select v-model="newAccount.group" placeholder="请选择分组" clearable style="width: 100%">
            <el-option label="未分组" value="" />
            <el-option v-for="g in groups" :key="g" :label="g" :value="g" />
          </el-select>
        </el-form-item>
        <el-divider />
        <el-form-item label="密钥" required>
          <el-input v-model="newAccount.secret" placeholder="Base32 编码密钥（必填）" />
        </el-form-item>
        <el-collapse v-model="addAdvancedVisible">
          <el-collapse-item title="⚙️ 高级选项" name="advanced">
            <el-alert type="info" :closable="false" show-icon style="margin-bottom: 16px">
              非必需，默认值适用于大多数情况
            </el-alert>
            <el-form-item label="算法">
              <el-select v-model="newAccount.algorithm" style="width: 100%">
                <el-option label="SHA1" value="SHA1" />
                <el-option label="SHA256" value="SHA256" />
                <el-option label="SHA512" value="SHA512" />
                <el-option label="MD5" value="MD5" />
              </el-select>
            </el-form-item>
            <el-form-item label="位数">
              <el-select v-model="newAccount.digits" style="width: 100%">
                <el-option :value="6" label="6 位" />
                <el-option :value="8" label="8 位" />
              </el-select>
            </el-form-item>
            <el-form-item label="周期">
              <el-input-number v-model="newAccount.period" :min="10" :max="120" :step="10" style="width: 100%" />
              <span style="font-size: 12px; color: #999; margin-left: 8px">秒</span>
            </el-form-item>
          </el-collapse-item>
        </el-collapse>
      </el-form>
      <template #footer>
        <el-button @click="addDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="addAccountManual">添加</el-button>
      </template>
    </el-dialog>

    <!-- 扫描二维码 -->
    <el-dialog v-model="scanDialogVisible" title="扫描二维码" width="400px" align-center>
      <p class="dialog-hint">支持标准 otpauth:// 格式的单个账户二维码</p>
      <div class="dialog-buttons">
        <el-button size="large" @click="importFromClipboard('standard')">
          📋 从剪贴板导入
        </el-button>
        <el-button size="large" @click="importFromFile('standard')">
          📁 选择图片文件
        </el-button>
      </div>
    </el-dialog>

    <!-- 转移验证码 - 导入 -->
    <el-dialog v-model="transferImportVisible" title="导入迁移码" width="400px" align-center>
      <p class="dialog-hint">支持 Google Authenticator 导出的批量迁移二维码</p>
      <div class="dialog-buttons">
        <el-button size="large" @click="importFromClipboard('migration')">
          📋 从剪贴板导入
        </el-button>
        <el-button size="large" @click="importFromFile('migration')">
          📁 选择图片文件
        </el-button>
      </div>
    </el-dialog>

    <!-- 转移验证码 - 导出 -->
    <el-dialog v-model="transferExportVisible" title="导出迁移码" width="500px" align-center>
      <div v-if="!exportQRCode">
        <p class="dialog-hint">选择要导出的账户，生成迁移二维码</p>
        <div class="export-select-all">
          <el-checkbox v-model="exportSelectAll" @change="toggleExportSelectAll">
            全选 ({{ accounts.length }} 个账户)
          </el-checkbox>
        </div>
        <div class="export-account-list">
          <el-checkbox-group v-model="exportSelectedAccounts">
            <el-checkbox v-for="acc in accounts" :key="acc.id" :value="acc.id">
              {{ acc.issuer || '未知' }} - {{ acc.name }}
            </el-checkbox>
          </el-checkbox-group>
        </div>
      </div>
      <div v-else class="export-qr-result">
        <img :src="exportQRCode" alt="迁移二维码" />
        <p>使用 Google Authenticator 扫描此二维码</p>
      </div>
      <template #footer>
        <el-button v-if="exportQRCode" @click="exportQRCode = ''">返回选择</el-button>
        <el-button v-if="!exportQRCode" @click="transferExportVisible = false">取消</el-button>
        <el-button v-if="!exportQRCode" type="primary" :disabled="exportSelectedAccounts.length === 0" @click="doExport">
          生成二维码
        </el-button>
        <el-button v-if="exportQRCode" type="primary" @click="transferExportVisible = false; exportQRCode = ''">完成</el-button>
      </template>
    </el-dialog>

    <!-- 新建分组 -->
    <el-dialog v-model="addGroupVisible" title="新建分组" width="360px" align-center>
      <el-input v-model="newGroupName" placeholder="输入分组名称" />
      <template #footer>
        <el-button @click="addGroupVisible = false">取消</el-button>
        <el-button type="primary" @click="createGroup">创建</el-button>
      </template>
    </el-dialog>

    <!-- 设置 -->
    <el-dialog v-model="settingsVisible" title="设置" width="420px" align-center>
      <el-form label-width="100px">
        <el-form-item label="主题">
          <el-radio-group v-model="theme">
            <el-radio value="light">浅色</el-radio>
            <el-radio value="dark">深色</el-radio>
            <el-radio value="auto">跟随系统</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-divider />
        <el-form-item label="密码保护">
          <el-switch v-model="passwordEnabled" @change="handlePasswordToggle" />
        </el-form-item>
        <el-form-item v-if="passwordEnabled" label="自动锁定">
          <el-select v-model="autoLockMinutes" @change="handleAutoLockChange" style="width: 160px">
            <el-option :value="0" label="不自动锁定" />
            <el-option :value="1" label="1 分钟" />
            <el-option :value="3" label="3 分钟" />
            <el-option :value="5" label="5 分钟" />
            <el-option :value="10" label="10 分钟" />
            <el-option :value="15" label="15 分钟" />
            <el-option :value="30" label="30 分钟" />
          </el-select>
        </el-form-item>
        <el-form-item v-if="passwordEnabled" label="修改密码">
          <el-button size="small" @click="changePasswordVisible = true">修改密码</el-button>
        </el-form-item>
      </el-form>
      <template #footer>
        <span class="setting-footer-hint">💡 关闭窗口会最小化到系统托盘</span>
      </template>
    </el-dialog>

    <!-- 设置密码 -->
    <el-dialog v-model="setPasswordVisible" title="设置密码" width="360px" align-center :close-on-click-modal="false">
      <el-form label-width="80px">
        <el-form-item label="新密码">
          <el-input v-model="newPassword" type="password" placeholder="请输入密码" show-password />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="confirmPassword" type="password" placeholder="再次输入密码" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="setPasswordVisible = false; passwordEnabled = false">取消</el-button>
        <el-button type="primary" @click="setPassword">确定</el-button>
      </template>
    </el-dialog>

    <!-- 修改密码 -->
    <el-dialog v-model="changePasswordVisible" title="修改密码" width="360px" align-center>
      <el-form label-width="80px">
        <el-form-item label="当前密码">
          <el-input v-model="currentPassword" type="password" placeholder="请输入当前密码" show-password />
        </el-form-item>
        <el-form-item label="新密码">
          <el-input v-model="newPassword" type="password" placeholder="请输入新密码" show-password />
        </el-form-item>
        <el-form-item label="确认密码">
          <el-input v-model="confirmPassword" type="password" placeholder="再次输入新密码" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="changePasswordVisible = false">取消</el-button>
        <el-button type="primary" @click="changePassword">确定</el-button>
      </template>
    </el-dialog>

    <!-- 关闭密码确认 -->
    <el-dialog v-model="disablePasswordVisible" title="关闭密码保护" width="360px" align-center :close-on-click-modal="false">
      <el-form label-width="80px">
        <el-form-item label="当前密码">
          <el-input v-model="currentPassword" type="password" placeholder="请输入当前密码" show-password />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="disablePasswordVisible = false; passwordEnabled = true">取消</el-button>
        <el-button type="primary" @click="disablePassword">确定</el-button>
      </template>
    </el-dialog>

    <!-- 编辑账户 -->
    <el-dialog v-model="editDialogVisible" title="编辑账户" width="480px" align-center>
      <el-form label-width="80px">
        <el-form-item label="账户名">
          <el-input v-model="editAccount.name" placeholder="user@example.com" />
        </el-form-item>
        <el-form-item label="发行者">
          <el-input v-model="editAccount.issuer" placeholder="Google" />
        </el-form-item>
        <el-form-item label="分组">
          <el-select v-model="editAccount.group" placeholder="请选择分组" clearable style="width: 100%">
            <el-option label="未分组" value="" />
            <el-option v-for="g in groups" :key="g" :label="g" :value="g" />
          </el-select>
        </el-form-item>
        <el-divider />
        <el-form-item label="密钥">
          <div style="display: flex; align-items: center; gap: 8px; width: 100%">
            <el-input value="••••••••••••••••" disabled style="flex: 1" />
            <el-button @click="openSecretDialog">🔍 查看</el-button>
          </div>
        </el-form-item>
        <el-collapse v-model="advancedVisible">
          <el-collapse-item title="⚙️ 高级选项" name="advanced">
            <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
              修改高级选项可能导致验证码错误，请谨慎操作
            </el-alert>
            <el-form-item label="算法">
              <el-select v-model="editAccount.algorithm" style="width: 100%">
                <el-option label="SHA1" value="SHA1" />
                <el-option label="SHA256" value="SHA256" />
                <el-option label="SHA512" value="SHA512" />
                <el-option label="MD5" value="MD5" />
              </el-select>
            </el-form-item>
            <el-form-item label="位数">
              <el-select v-model="editAccount.digits" style="width: 100%">
                <el-option :value="6" label="6 位" />
                <el-option :value="8" label="8 位" />
              </el-select>
            </el-form-item>
            <el-form-item label="周期">
              <el-input-number v-model="editAccount.period" :min="10" :max="120" :step="10" style="width: 100%" />
              <span style="font-size: 12px; color: #999; margin-left: 8px">秒</span>
            </el-form-item>
          </el-collapse-item>
        </el-collapse>
      </el-form>
      <template #footer>
        <el-button @click="editDialogVisible = false">取消</el-button>
        <el-button type="primary" @click="saveAccountEdit">保存</el-button>
      </template>
    </el-dialog>

    <!-- 查看密钥 -->
    <el-dialog v-model="viewSecretVisible" title="⚠️ 查看敏感信息" width="400px" align-center>
      <el-alert type="warning" :closable="false" show-icon style="margin-bottom: 16px">
        密钥明文将显示，请确保四处无人
      </el-alert>
      <el-form v-if="!viewedSecret" label-width="80px">
        <el-form-item label="密码">
          <el-input
            v-model="secretPassword"
            type="password"
            placeholder="请输入密码验证身份"
            show-password
            @keyup.enter="viewSecret"
          />
        </el-form-item>
      </el-form>
      <div v-else class="secret-view">
        <el-form-item label="密钥">
          <div style="display: flex; align-items: center; gap: 8px">
            <el-input :value="viewedSecret" readonly style="font-family: monospace; font-size: 14px" />
            <el-button @click="copySecret">📋 复制</el-button>
          </div>
        </el-form-item>
      </div>
      <template #footer>
        <el-button v-if="!viewedSecret" @click="closeSecretView">取消</el-button>
        <el-button v-if="!viewedSecret" type="primary" @click="viewSecret">查看</el-button>
        <el-button v-else @click="closeSecretView">关闭</el-button>
      </template>
    </el-dialog>

    <!-- 关于 -->
    <el-dialog v-model="aboutVisible" title="关于" width="360px" align-center>
      <div class="about-content">
        <div class="about-icon">🔐</div>
            <h2>Google Authenticator</h2>
        <p class="version">桌面版 v1.0.0</p>
        <el-divider />
        <p>基于 Wails + Vue 3 + Element Plus</p>
        <p class="copyright">By RabbitMask © 2025</p>
      </div>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onUnmounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { Search, Plus, Refresh, CopyDocument, ArrowDown, Edit } from '@element-plus/icons-vue'
import {
  GetAllAccounts,
  GenerateCode,
  ImportFromQRCodeImage,
  ImportFromFile,
  ExportToMigrationQR,
  AddAccountWithGroup,
  DeleteAccounts,
  GetGroups,
  UpdateAccountsGroup,
  UpdateAccount,
  UpdateAccountAdvanced,
  GetAccountSecret,
  GetSettings,
  SetTheme,
  EnablePassword,
  DisablePassword,
  ChangePassword,
  VerifyPassword,
  IsPasswordEnabled,
  SetAutoLockMinutes,
  GetAutoLockMinutes,
  Unlock,
  NeedsUnlock
} from '../wailsjs/go/main/App'
import { EventsOn } from '../wailsjs/runtime/runtime'

// ========== 状态 ==========
const accounts = ref([])
const codes = ref({})
const groups = ref([])
const searchQuery = ref('')
const currentGroup = ref('')
const selectedAccounts = ref([])

// 锁屏相关
const isLocked = ref(false)
const unlockPassword = ref('')

// 密码管理相关
const passwordEnabled = ref(false)
const setPasswordVisible = ref(false)
const changePasswordVisible = ref(false)
const disablePasswordVisible = ref(false)
const currentPassword = ref('')
const newPassword = ref('')
const confirmPassword = ref('')

// 自动锁定
const autoLockMinutes = ref(5)
let autoLockTimer = null
let lastActivityTime = Date.now()

// 对话框
const addChoiceVisible = ref(false)
const addDialogVisible = ref(false)
const scanDialogVisible = ref(false)
const transferImportVisible = ref(false)
const transferExportVisible = ref(false)
const addGroupVisible = ref(false)
const settingsVisible = ref(false)
const aboutVisible = ref(false)
const editDialogVisible = ref(false)
const viewSecretVisible = ref(false)
const advancedVisible = ref([])
const addAdvancedVisible = ref([])

// 导出相关
const exportSelectedAccounts = ref([])
const exportSelectAll = ref(false)
const exportQRCode = ref('')

// 新建分组
const newGroupName = ref('')

// 设置
const theme = ref('light')

// 新账户表单
const newAccount = ref({
  name: '',
  issuer: '',
  secret: '',
  algorithm: 'SHA1',
  digits: 6,
  period: 30,
  group: ''
})

// 编辑账户表单
const editAccount = ref({
  id: '',
  name: '',
  issuer: '',
  group: '',
  algorithm: 'SHA1',
  digits: 6,
  period: 30
})

// 查看密钥
const secretPassword = ref('')
const viewedSecret = ref('')

// ========== 计算属性 ==========
const filteredAccounts = computed(() => {
  let list = accounts.value

  // 按分组筛选
  if (currentGroup.value) {
    list = list.filter(a => a.group === currentGroup.value)
  }

  // 按搜索词筛选
  if (searchQuery.value) {
    const q = searchQuery.value.toLowerCase()
    list = list.filter(a =>
      a.name?.toLowerCase().includes(q) ||
      a.issuer?.toLowerCase().includes(q)
    )
  }

  return list
})

// ========== 方法 ==========
async function loadAccounts() {
  try {
    accounts.value = await GetAllAccounts() || []
    groups.value = await GetGroups() || []
    await updateCodes()
  } catch (e) {
    console.error('加载账户失败:', e)
  }
}

async function updateCodes() {
  for (const acc of accounts.value) {
    try {
      codes.value[acc.id] = await GenerateCode(acc.id)
    } catch (e) {
      console.error('生成验证码失败:', e)
    }
  }
}

function formatCode(code) {
  if (!code || code === '------') return '--- ---'
  if (code.length === 6) return code.slice(0, 3) + ' ' + code.slice(3)
  if (code.length === 8) return code.slice(0, 4) + ' ' + code.slice(4)
  return code
}

function getTimeColor(remaining) {
  if (!remaining) return '#67c23a'
  if (remaining <= 5) return '#f56c6c'
  if (remaining <= 10) return '#e6a23c'
  return '#67c23a'
}

function getGroupCount(group) {
  return accounts.value.filter(a => a.group === group).length
}

async function copyCode(account) {
  const code = codes.value[account.id]?.code
  if (!code || code === '------' || code === 'ERROR') return
  try {
    await navigator.clipboard.writeText(code)
    ElMessage.success(`已复制: ${code}`)
  } catch {
    ElMessage.error('复制失败')
  }
}

function toggleSelect(id) {
  const idx = selectedAccounts.value.indexOf(id)
  if (idx > -1) {
    selectedAccounts.value.splice(idx, 1)
  } else {
    selectedAccounts.value.push(id)
  }
}

function showAddDialog() {
  addChoiceVisible.value = true
}

function showAddGroupDialog() {
  newGroupName.value = ''
  addGroupVisible.value = true
}

async function createGroup() {
  if (!newGroupName.value.trim()) {
    ElMessage.warning('请输入分组名称')
    return
  }
  groups.value.push(newGroupName.value.trim())
  addGroupVisible.value = false
  ElMessage.success('分组创建成功')
}

async function moveToGroup(group) {
  if (group === '__new__') {
    showAddGroupDialog()
    return
  }

  try {
    await UpdateAccountsGroup(selectedAccounts.value, group)
    selectedAccounts.value = []
    await loadAccounts()
    ElMessage.success(group ? `已移动到「${group}」` : '已移至未分组')
  } catch (e) {
    ElMessage.error('移动失败')
  }
}

async function deleteSelected() {
  if (selectedAccounts.value.length === 0) return

  try {
    await ElMessageBox.confirm(
      `确定删除选中的 ${selectedAccounts.value.length} 个账户？`,
      '删除确认',
      { type: 'warning' }
    )
    await DeleteAccounts(selectedAccounts.value)
    selectedAccounts.value = []
    await loadAccounts()
    ElMessage.success('删除成功')
  } catch {}
}

// 打开编辑对话框
function openEditDialog(account) {
  editAccount.value = {
    id: account.id,
    name: account.name,
    issuer: account.issuer,
    group: account.group || '',
    algorithm: account.algorithm,
    digits: account.digits,
    period: account.period || 30
  }
  advancedVisible.value = []
  editDialogVisible.value = true
}

function openSecretDialog() {
  viewedSecret.value = ''
  secretPassword.value = ''
  viewSecretVisible.value = true
}

// 保存账户编辑
async function saveAccountEdit() {
  try {
    // 保存基础信息
    const basicSuccess = await UpdateAccount(
      editAccount.value.id,
      editAccount.value.name,
      editAccount.value.issuer,
      editAccount.value.group
    )

    if (!basicSuccess) {
      ElMessage.error('保存失败')
      return
    }

    // 如果修改了高级选项，需要二次确认
    if (advancedVisible.value.includes('advanced')) {
      await ElMessageBox.confirm(
        '确定要修改高级选项吗？这可能导致验证码错误',
        '确认修改',
        { type: 'warning' }
      )

      const advancedSuccess = await UpdateAccountAdvanced(
        editAccount.value.id,
        editAccount.value.algorithm,
        editAccount.value.digits,
        editAccount.value.period
      )

      if (!advancedSuccess) {
        ElMessage.error('高级选项保存失败')
        return
      }
    }

    editDialogVisible.value = false
    await loadAccounts()
    ElMessage.success('保存成功')
  } catch (e) {
    if (e !== 'cancel') {
      ElMessage.error('操作失败')
    }
  }
}

// 查看密钥
async function viewSecret() {
  try {
    const secret = await GetAccountSecret(editAccount.value.id, secretPassword.value)
    if (!secret) {
      ElMessage.error('密码错误或账户不存在')
      return
    }
    viewedSecret.value = secret
    secretPassword.value = ''
  } catch (e) {
    ElMessage.error('获取密钥失败')
  }
}

// 关闭密钥查看
function closeSecretView() {
  viewSecretVisible.value = false
  viewedSecret.value = ''
  secretPassword.value = ''
}

// 复制密钥
function copySecret() {
  navigator.clipboard.writeText(viewedSecret.value)
  ElMessage.success('密钥已复制')
}


async function addAccountManual() {
  if (!newAccount.value.name || !newAccount.value.secret) {
    ElMessage.warning('请填写账户名和密钥')
    return
  }

  try {
    const result = await AddAccountWithGroup(
      newAccount.value.name,
      newAccount.value.issuer,
      newAccount.value.secret.toUpperCase().replace(/\s/g, ''),
      newAccount.value.algorithm,
      'TOTP',
      newAccount.value.digits,
      newAccount.value.period,
      newAccount.value.group
    )

    if (result.success) {
      ElMessage.success('账户添加成功')
      addDialogVisible.value = false
      addAdvancedVisible.value = []
      newAccount.value = {
        name: '',
        issuer: '',
        secret: '',
        algorithm: 'SHA1',
        digits: 6,
        period: 30,
        group: ''
      }
      await loadAccounts()
    } else {
      ElMessage.error(result.message)
    }
  } catch (e) {
    ElMessage.error('添加失败')
  }
}

async function importFromClipboard(type) {
  try {
    const items = await navigator.clipboard.read()
    for (const item of items) {
      const imageType = item.types.find(t => t.startsWith('image/'))
      if (imageType) {
        const blob = await item.getType(imageType)
        const reader = new FileReader()
        reader.onload = async (e) => {
          const result = await ImportFromQRCodeImage(e.target.result)
          handleImportResult(result, type)
        }
        reader.readAsDataURL(blob)
        return
      }
    }
    ElMessage.warning('剪贴板中没有图片')
  } catch (e) {
    ElMessage.error('读取剪贴板失败')
  }
}

async function importFromFile(type) {
  try {
    const result = await ImportFromFile()
    handleImportResult(result, type)
  } catch (e) {
    ElMessage.error('导入失败')
  }
}

function handleImportResult(result, type) {
  if (result.success) {
    ElMessage.success(result.message)
    loadAccounts()
    scanDialogVisible.value = false
    transferImportVisible.value = false
  } else if (result.message !== '未选择文件') {
    ElMessage.error(result.message)
  }
}

function toggleExportSelectAll(val) {
  if (val) {
    exportSelectedAccounts.value = accounts.value.map(a => a.id)
  } else {
    exportSelectedAccounts.value = []
  }
}

async function doExport() {
  if (exportSelectedAccounts.value.length === 0) {
    ElMessage.warning('请选择要导出的账户')
    return
  }

  try {
    const result = await ExportToMigrationQR(exportSelectedAccounts.value, 400)
    if (result.success) {
      exportQRCode.value = result.qr_code_url
    } else {
      ElMessage.error(result.message)
    }
  } catch (e) {
    ElMessage.error('导出失败')
  }
}

function selectAll() {
  if (selectedAccounts.value.length === filteredAccounts.value.length) {
    selectedAccounts.value = []
  } else {
    selectedAccounts.value = filteredAccounts.value.map(a => a.id)
  }
}

// ========== 密码管理 ==========
async function checkPasswordProtection() {
  try {
    const enabled = await IsPasswordEnabled()
    passwordEnabled.value = enabled

    // 检查是否需要解锁（有密码但未解锁）
    const needsUnlock = await NeedsUnlock()
    if (needsUnlock) {
      isLocked.value = true
    }
  } catch (e) {
    console.error('检查密码状态失败:', e)
  }
}

async function unlock() {
  if (!unlockPassword.value) {
    ElMessage.warning('请输入密码')
    return
  }
  try {
    // 调用 Unlock 来真正解锁数据库并设置 masterKey
    const result = await Unlock(unlockPassword.value)
    if (result) {
      isLocked.value = false
      unlockPassword.value = ''
      // 解锁后重新加载数据
      await loadAccounts()
    } else {
      ElMessage.error('密码错误')
    }
  } catch (e) {
    ElMessage.error('验证失败')
  }
}

function handlePasswordToggle(val) {
  if (val) {
    // 开启密码保护
    setPasswordVisible.value = true
  } else {
    // 关闭密码保护
    disablePasswordVisible.value = true
  }
}

async function setPassword() {
  if (!newPassword.value) {
    ElMessage.warning('请输入密码')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  try {
    const result = await EnablePassword(newPassword.value)
    if (result) {
      ElMessage.success('密码设置成功')
      setPasswordVisible.value = false
      newPassword.value = ''
      confirmPassword.value = ''
    } else {
      ElMessage.error('设置失败')
      passwordEnabled.value = false
    }
  } catch (e) {
    ElMessage.error('设置失败')
    passwordEnabled.value = false
  }
}

async function disablePassword() {
  if (!currentPassword.value) {
    ElMessage.warning('请输入当前密码')
    return
  }
  try {
    const result = await DisablePassword(currentPassword.value)
    if (result) {
      ElMessage.success('密码保护已关闭')
      disablePasswordVisible.value = false
      currentPassword.value = ''
    } else {
      ElMessage.error('密码错误')
      passwordEnabled.value = true
    }
  } catch (e) {
    ElMessage.error('操作失败')
    passwordEnabled.value = true
  }
}

async function changePassword() {
  if (!currentPassword.value) {
    ElMessage.warning('请输入当前密码')
    return
  }
  if (!newPassword.value) {
    ElMessage.warning('请输入新密码')
    return
  }
  if (newPassword.value !== confirmPassword.value) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  try {
    const result = await ChangePassword(currentPassword.value, newPassword.value)
    if (result) {
      ElMessage.success('密码修改成功')
      changePasswordVisible.value = false
      currentPassword.value = ''
      newPassword.value = ''
      confirmPassword.value = ''
    } else {
      ElMessage.error('当前密码错误')
    }
  } catch (e) {
    ElMessage.error('修改失败')
  }
}

// ========== 自动锁定 ==========
async function handleAutoLockChange(val) {
  try {
    await SetAutoLockMinutes(val)
    resetAutoLockTimer()
  } catch (e) {
    ElMessage.error('设置自动锁定失败')
  }
}

// ========== 自动锁定 ==========
function resetAutoLockTimer() {
  lastActivityTime = Date.now()
}

function checkAutoLock() {
  if (!passwordEnabled.value || isLocked.value || autoLockMinutes.value === 0) {
    return
  }

  const idleTime = Date.now() - lastActivityTime
  const lockTime = autoLockMinutes.value * 60 * 1000

  if (idleTime >= lockTime) {
    isLocked.value = true
  }
}

function setupActivityListeners() {
  const events = ['mousedown', 'mousemove', 'keydown', 'scroll', 'touchstart', 'click']
  events.forEach(event => {
    document.addEventListener(event, resetAutoLockTimer, { passive: true })
  })
}

async function loadSettings() {
  try {
    const settings = await GetSettings()
    autoLockMinutes.value = settings.auto_lock_minutes || 5
  } catch (e) {
    console.error('加载设置失败:', e)
  }
}

// ========== 生命周期 ==========
let timer = null

// 应用主题
function applyTheme(themeName) {
  const html = document.documentElement
  if (themeName === 'auto') {
    const prefersDark = window.matchMedia('(prefers-color-scheme: dark)').matches
    html.setAttribute('data-theme', prefersDark ? 'dark' : 'light')
  } else {
    html.setAttribute('data-theme', themeName)
  }
  localStorage.setItem('theme', themeName)
}

// 监听主题变化
watch(theme, (newTheme) => {
  applyTheme(newTheme)
})

onMounted(async () => {
  // 加载设置
  await loadSettings()

  // 检查密码保护状态
  await checkPasswordProtection()

  // 加载保存的主题
  const savedTheme = localStorage.getItem('theme') || 'light'
  theme.value = savedTheme
  applyTheme(savedTheme)

  // 监听系统主题变化
  window.matchMedia('(prefers-color-scheme: dark)').addEventListener('change', () => {
    if (theme.value === 'auto') {
      applyTheme('auto')
    }
  })

  await loadAccounts()
  timer = setInterval(updateCodes, 1000)

  // 设置自动锁定检查（每10秒检查一次）
  autoLockTimer = setInterval(checkAutoLock, 10000)
  setupActivityListeners()

  // 菜单事件监听
  EventsOn('menu:add-manual', () => { addDialogVisible.value = true })
  EventsOn('menu:scan-qr', () => { scanDialogVisible.value = true })
  EventsOn('menu:transfer-import', () => { transferImportVisible.value = true })
  EventsOn('menu:transfer-export', () => {
    exportSelectedAccounts.value = []
    exportSelectAll.value = false
    exportQRCode.value = ''
    transferExportVisible.value = true
  })
  EventsOn('menu:select-all', selectAll)
  EventsOn('menu:settings', () => { settingsVisible.value = true })
  EventsOn('menu:about', () => { aboutVisible.value = true })
})

onUnmounted(() => {
  if (timer) clearInterval(timer)
  if (autoLockTimer) clearInterval(autoLockTimer)
})

// 监听导出选择变化
watch(exportSelectedAccounts, (val) => {
  exportSelectAll.value = val.length === accounts.value.length && accounts.value.length > 0
})
</script>

<style scoped>
.app-container {
  width: 100%;
  height: 100vh;
  background:
    radial-gradient(circle at top left, rgba(194, 145, 59, 0.12), transparent 28%),
    linear-gradient(180deg, #f7f2e8 0%, #efe7d8 100%);
  display: flex;
  flex-direction: column;
}

/* ========== 锁屏界面 ========== */
.lock-screen {
  position: fixed;
  top: 0;
  left: 0;
  right: 0;
  bottom: 0;
  background:
    radial-gradient(circle at 20% 20%, rgba(214, 170, 78, 0.22), transparent 26%),
    radial-gradient(circle at 80% 15%, rgba(138, 162, 118, 0.18), transparent 24%),
    linear-gradient(135deg, #20352f 0%, #14231f 55%, #101917 100%);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 9999;
}

.lock-content {
  text-align: center;
  color: white;
  padding: 40px;
  background: rgba(252, 247, 238, 0.08);
  border: 1px solid rgba(231, 213, 178, 0.18);
  border-radius: 20px;
  box-shadow: 0 24px 60px rgba(0, 0, 0, 0.24);
  backdrop-filter: blur(14px);
}

.lock-icon {
  font-size: 64px;
  margin-bottom: 16px;
}

.lock-content h2 {
  margin: 0 0 8px;
  font-size: 28px;
}

.lock-content p {
  margin: 0;
  opacity: 0.9;
  font-size: 14px;
}

/* ========== 空状态 ========== */
.empty-welcome {
  flex: 1;
  display: flex;
  align-items: center;
  justify-content: center;
  background:
    radial-gradient(circle at top center, rgba(201, 156, 65, 0.18), transparent 24%),
    linear-gradient(160deg, #f6efe3 0%, #efe5d2 46%, #e5d8c2 100%);
}

.welcome-content {
  text-align: center;
  color: #24352f;
  padding: 56px 52px;
  border: 1px solid rgba(92, 76, 45, 0.08);
  border-radius: 28px;
  background: rgba(255, 251, 245, 0.7);
  box-shadow: 0 30px 80px rgba(68, 54, 30, 0.12);
  backdrop-filter: blur(10px);
}

.welcome-icon {
  font-size: 80px;
  margin-bottom: 16px;
}

.welcome-content h1 {
  font-size: 36px;
  margin: 0 0 8px;
  letter-spacing: 0.03em;
}

.subtitle {
  font-size: 18px;
  color: rgba(36, 53, 47, 0.72);
  margin: 0 0 40px;
}

.welcome-actions {
  display: flex;
  gap: 16px;
  justify-content: center;
  margin-bottom: 24px;
}

.welcome-actions .el-button {
  min-width: 160px;
  height: 52px;
  font-size: 16px;
  font-weight: 700;
  border-radius: 14px;
}

.welcome-tip {
  font-size: 14px;
  color: rgba(36, 53, 47, 0.66);
}

/* ========== 主布局 ========== */
.main-layout {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

/* 顶部工具栏 */
.top-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: rgba(255, 251, 245, 0.88);
  border-bottom: 1px solid #ddd0bb;
  backdrop-filter: blur(10px);
  box-shadow: 0 8px 24px rgba(79, 61, 33, 0.05);
}

.toolbar-left {
  display: flex;
  align-items: center;
  gap: 16px;
}

.toolbar-brand {
  font-size: 18px;
  font-weight: 600;
  color: #24352f;
  white-space: nowrap;
  letter-spacing: 0.02em;
}

.toolbar-search {
  width: 280px;
}

.toolbar-search :deep(.el-input__wrapper) {
  border-radius: 14px;
}

/* 主体区域 */
.main-body {
  flex: 1;
  display: flex;
  overflow: hidden;
}

/* 左侧分组栏 */
.sidebar {
  width: 200px;
  background: rgba(255, 250, 243, 0.8);
  border-right: 1px solid #ddd0bb;
  padding: 16px 0;
  overflow-y: auto;
}

.sidebar-title {
  padding: 0 16px 12px;
  font-size: 12px;
  color: #8b7961;
  font-weight: 500;
  letter-spacing: 0.08em;
}

.group-item {
  display: flex;
  align-items: center;
  padding: 10px 16px;
  cursor: pointer;
  transition: background 0.2s;
  color: #33463f;
  border-left: 3px solid transparent;
}

.group-item:hover {
  background: rgba(194, 145, 59, 0.08);
}

.group-item.active {
  background: rgba(36, 53, 47, 0.08);
  color: #25493e;
  border-left-color: #b98133;
}

.group-icon {
  margin-right: 8px;
}

.group-name {
  flex: 1;
  font-size: 14px;
  color: inherit;
}

.group-count {
  font-size: 12px;
  color: #7e6d58;
  background: #efe4d1;
  padding: 2px 8px;
  border-radius: 10px;
}

.group-item.active .group-count {
  background: #e5d3b4;
  color: #8f5a1f;
}

.group-item.add-group {
  color: #8f5a1f;
  margin-top: 8px;
}

/* 右侧内容区 */
.content-area {
  flex: 1;
  display: flex;
  flex-direction: column;
  overflow: hidden;
}

.info-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: rgba(255, 251, 245, 0.72);
  border-bottom: 1px solid #ddd0bb;
}

.account-count {
  font-size: 14px;
  color: #6f6559;
}

/* 账户列表 */
.accounts-list {
  flex: 1;
  overflow-y: auto;
  padding: 16px 20px;
}

.account-item {
  display: flex;
  align-items: center;
  background: rgba(255, 251, 245, 0.82);
  border-radius: 14px;
  padding: 16px 20px;
  margin-bottom: 12px;
  cursor: pointer;
  transition: all 0.2s;
  border: 1px solid rgba(178, 154, 118, 0.18);
  position: relative;
  box-shadow: 0 10px 30px rgba(78, 61, 35, 0.06);
}

.edit-btn {
  position: absolute;
  top: 8px;
  right: 8px;
  font-size: 16px;
  color: #909399;
  cursor: pointer;
  opacity: 0;
  transition: all 0.2s;
  padding: 4px;
  border-radius: 4px;
  z-index: 10;
}

.account-item:hover .edit-btn {
  opacity: 1;
}

.edit-btn:hover {
  color: #8f5a1f;
  background: rgba(194, 145, 59, 0.12);
}

.account-item:hover {
  transform: translateY(-1px);
  box-shadow: 0 16px 36px rgba(78, 61, 35, 0.12);
}

.account-item.selected {
  border-color: rgba(191, 134, 58, 0.55);
  background: linear-gradient(180deg, rgba(255, 249, 238, 0.96) 0%, rgba(244, 232, 210, 0.96) 100%);
  box-shadow: 0 18px 40px rgba(143, 90, 31, 0.12);
}

.account-left {
  display: flex;
  align-items: center;
  flex: 1;
  min-width: 0;
}

.account-icon {
  font-size: 24px;
  margin-right: 12px;
}

.account-info {
  min-width: 0;
}

.account-issuer {
  font-size: 16px;
  font-weight: 600;
  color: #24352f;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.account-group {
  font-size: 13px;
  font-weight: 400;
  color: #8b7961;
  margin-left: 2px;
}

.account-name {
  font-size: 13px;
  color: #8b7961;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}

.account-center {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px 16px;
  background:
    linear-gradient(135deg, #27443d 0%, #1f352f 55%, #172824 100%);
  border: 1px solid rgba(226, 200, 151, 0.2);
  border-radius: 12px;
  cursor: pointer;
  transition: transform 0.15s;
  box-shadow: inset 0 1px 0 rgba(255, 248, 230, 0.08);
}

.account-center:hover {
  transform: scale(1.02);
}

.code-text {
  font-size: 28px;
  font-weight: 700;
  color: white;
  font-family: 'Courier New', monospace;
  letter-spacing: 2px;
}

.copy-icon {
  color: rgba(255, 255, 255, 0.8);
  font-size: 18px;
}

.account-right {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-left: 20px;
}

.time-text {
  font-size: 16px;
  font-weight: 600;
  min-width: 32px;
  text-align: right;
}

.no-accounts {
  text-align: center;
  padding: 60px 20px;
  color: #909399;
}

/* 底部选择栏 */
.selection-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 20px;
  background: linear-gradient(90deg, #24352f 0%, #1a2723 100%);
  color: white;
}

.selection-info {
  font-size: 14px;
}

.selection-actions {
  display: flex;
  gap: 10px;
}

.selection-actions .el-button {
  border-radius: 12px;
}

.slide-up-enter-active,
.slide-up-leave-active {
  transition: all 0.25s ease;
}

.slide-up-enter-from,
.slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}

/* ========== 对话框 ========== */
/* 统一按钮样式 */
.dialog-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 0 20px;
}

.dialog-buttons .el-button {
  width: 100%;
  height: 52px;
  font-size: 15px;
  border-radius: 14px;
  margin: 0;
}

.add-choice-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 0 20px;
}

.add-choice-buttons .el-button {
  width: 100%;
  height: 52px;
  font-size: 15px;
  border-radius: 14px;
  margin: 0;
}

.dialog-hint {
  color: #8b7961;
  font-size: 14px;
  margin-bottom: 20px;
  text-align: center;
}

.scan-buttons {
  display: flex;
  flex-direction: column;
  gap: 12px;
  padding: 0 20px;
}

.scan-buttons .el-button {
  width: 100%;
  height: 52px;
  font-size: 15px;
  border-radius: 14px;
  margin: 0;
}

/* 设置底部提示 */
.setting-footer-hint {
  font-size: 12px;
  color: #8b7961;
}

.export-select-all {
  margin-bottom: 16px;
}

.export-account-list {
  max-height: 300px;
  overflow-y: auto;
  border: 1px solid #ddd0bb;
  border-radius: 10px;
  padding: 12px;
  background: rgba(255, 251, 245, 0.75);
}

.export-account-list .el-checkbox {
  display: block;
  margin-bottom: 8px;
}

.export-qr-result {
  text-align: center;
}

.export-qr-result img {
  width: 280px;
  height: 280px;
  margin-bottom: 12px;
}

.export-qr-result p {
  color: #6f6559;
}

.about-content {
  text-align: center;
  padding: 20px 0;
}

.about-icon {
  font-size: 64px;
  margin-bottom: 12px;
}

.about-content h2 {
  margin: 0 0 4px;
  font-size: 22px;
  color: #24352f;
}

.version {
  color: #8b7961;
  margin: 0;
}

.copyright {
  color: #b4a48f;
  font-size: 12px;
}

/* 滚动条 */
.accounts-list::-webkit-scrollbar,
.sidebar::-webkit-scrollbar {
  width: 6px;
}

.accounts-list::-webkit-scrollbar-thumb,
.sidebar::-webkit-scrollbar-thumb {
  background: #cdbda5;
  border-radius: 3px;
}
</style>

<!-- 全局样式（包含主题变量） -->
<style>
/* 浅色主题（默认） */
:root,
[data-theme="light"] {
  --bg-primary: #efe7d8;
  --bg-secondary: rgba(255, 251, 245, 0.88);
  --bg-card: rgba(255, 251, 245, 0.82);
  --text-primary: #24352f;
  --text-secondary: #5d665f;
  --text-muted: #8b7961;
  --border-color: #ddd0bb;
  --hover-bg: rgba(194, 145, 59, 0.08);
  --active-bg: rgba(36, 53, 47, 0.08);
  --active-color: #8f5a1f;
}

/* 深色主题 */
[data-theme="dark"] {
  --bg-primary: #16211f;
  --bg-secondary: #1d2b28;
  --bg-card: #213330;
  --text-primary: #f1e8d8;
  --text-secondary: #d3c5b1;
  --text-muted: #a99983;
  --border-color: #314542;
  --hover-bg: rgba(201, 156, 65, 0.12);
  --active-bg: rgba(201, 156, 65, 0.16);
  --active-color: #d6aa4e;
}

/* 应用主题变量 */
[data-theme="dark"] .app-container {
  background: var(--bg-primary);
}

[data-theme="dark"] .top-toolbar,
[data-theme="dark"] .sidebar,
[data-theme="dark"] .info-bar {
  background: var(--bg-secondary);
  border-color: var(--border-color);
}

[data-theme="dark"] .toolbar-brand,
[data-theme="dark"] .account-issuer,
[data-theme="dark"] .about-content h2 {
  color: var(--text-primary);
}

[data-theme="dark"] .account-group {
  color: var(--text-muted);
}

[data-theme="dark"] .account-name,
[data-theme="dark"] .account-count,
[data-theme="dark"] .sidebar-title {
  color: var(--text-muted);
}

[data-theme="dark"] .group-item:hover {
  background: var(--hover-bg);
}

[data-theme="dark"] .group-item.active {
  background: var(--active-bg);
  color: var(--active-color);
}

[data-theme="dark"] .group-count {
  background: var(--bg-card);
  color: var(--text-muted);
}

[data-theme="dark"] .group-item.active .group-count {
  background: var(--active-bg);
  color: var(--active-color);
}

[data-theme="dark"] .account-item {
  background: var(--bg-card);
}

[data-theme="dark"] .account-item:hover {
  box-shadow: 0 18px 38px rgba(0, 0, 0, 0.34);
}

[data-theme="dark"] .account-item.selected {
  border-color: var(--active-color);
  background: var(--active-bg);
}

[data-theme="dark"] .no-accounts,
[data-theme="dark"] .version {
  color: var(--text-muted);
}

[data-theme="dark"] .accounts-list::-webkit-scrollbar-thumb,
[data-theme="dark"] .sidebar::-webkit-scrollbar-thumb {
  background: var(--border-color);
}

/* Element Plus 深色主题适配 */
[data-theme="dark"] .el-input__wrapper {
  background: var(--bg-card);
  box-shadow: 0 0 0 1px var(--border-color) inset;
}

[data-theme="dark"] .el-input__inner {
  color: var(--text-primary);
}

[data-theme="dark"] .el-dialog {
  background: var(--bg-secondary);
}

[data-theme="dark"] .el-dialog__title {
  color: var(--text-primary);
}

[data-theme="dark"] .el-form-item__label {
  color: var(--text-secondary);
}

:deep(.el-button) {
  font-weight: 600;
  letter-spacing: 0.01em;
}

:deep(.el-button--primary) {
  border-color: #8f5a1f;
  background: linear-gradient(135deg, #a06a29 0%, #8f5a1f 58%, #764715 100%);
  box-shadow: 0 10px 24px rgba(143, 90, 31, 0.22);
}

:deep(.el-button--primary:hover) {
  border-color: #b98133;
  background: linear-gradient(135deg, #b47831 0%, #9b6120 60%, #7e4d18 100%);
}

:deep(.el-button:not(.el-button--primary):not(.el-button--danger)) {
  border-color: #d8cab3;
  background: rgba(255, 250, 243, 0.88);
  color: #41524b;
}

:deep(.el-button:not(.el-button--primary):not(.el-button--danger):hover) {
  border-color: #c99c41;
  color: #8f5a1f;
  background: rgba(243, 234, 219, 0.92);
}

:deep(.el-button--danger) {
  box-shadow: 0 10px 20px rgba(182, 84, 58, 0.18);
}

:deep(.el-input__wrapper),
:deep(.el-select__wrapper),
:deep(.el-textarea__inner) {
  background: rgba(255, 251, 245, 0.92);
  box-shadow: 0 0 0 1px rgba(192, 173, 145, 0.5) inset;
  border-radius: 12px;
}

:deep(.el-input__wrapper:hover),
:deep(.el-select__wrapper:hover) {
  box-shadow: 0 0 0 1px rgba(185, 129, 51, 0.5) inset;
}

:deep(.el-input__wrapper.is-focus),
:deep(.el-select__wrapper.is-focused),
:deep(.el-textarea__inner:focus) {
  box-shadow: 0 0 0 1px #b98133 inset, 0 0 0 4px rgba(201, 156, 65, 0.14);
}

:deep(.el-dialog) {
  border: 1px solid rgba(197, 176, 144, 0.45);
  border-radius: 22px;
  background:
    linear-gradient(180deg, rgba(255, 252, 247, 0.98) 0%, rgba(249, 243, 234, 0.98) 100%);
  box-shadow: 0 32px 80px rgba(56, 43, 21, 0.18);
  overflow: hidden;
}

:deep(.el-dialog__header) {
  margin-right: 0;
  padding: 22px 24px 14px;
  border-bottom: 1px solid rgba(214, 198, 172, 0.5);
  background: rgba(255, 249, 241, 0.9);
}

:deep(.el-dialog__title) {
  color: #24352f;
  font-weight: 700;
  letter-spacing: 0.02em;
}

:deep(.el-dialog__body) {
  padding: 20px 24px;
}

:deep(.el-dialog__footer) {
  padding: 14px 24px 22px;
  border-top: 1px solid rgba(214, 198, 172, 0.42);
  background: rgba(255, 250, 244, 0.78);
}

:deep(.el-divider__text),
:deep(.el-form-item__label) {
  color: #6f6559;
}

:deep(.el-radio__label),
:deep(.el-checkbox__label) {
  color: #41524b;
}

[data-theme="dark"] :deep(.el-button:not(.el-button--primary):not(.el-button--danger)) {
  border-color: #3d4f49;
  background: rgba(34, 50, 46, 0.92);
  color: #e0d4c1;
}

[data-theme="dark"] :deep(.el-button:not(.el-button--primary):not(.el-button--danger):hover) {
  border-color: #d6aa4e;
  color: #f1e8d8;
  background: rgba(52, 72, 66, 0.96);
}

[data-theme="dark"] :deep(.el-dialog) {
  border-color: rgba(72, 95, 88, 0.8);
  background: linear-gradient(180deg, rgba(28, 42, 39, 0.98) 0%, rgba(24, 35, 33, 0.98) 100%);
  box-shadow: 0 36px 90px rgba(0, 0, 0, 0.4);
}

[data-theme="dark"] :deep(.el-dialog__header),
[data-theme="dark"] :deep(.el-dialog__footer) {
  background: rgba(30, 44, 40, 0.88);
  border-color: rgba(72, 95, 88, 0.7);
}

[data-theme="dark"] :deep(.el-dialog__title),
[data-theme="dark"] :deep(.el-divider__text),
[data-theme="dark"] :deep(.el-radio__label),
[data-theme="dark"] :deep(.el-checkbox__label) {
  color: #f1e8d8;
}
</style>

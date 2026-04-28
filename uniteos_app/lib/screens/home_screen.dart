import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/sync_provider.dart';
import '../main.dart';

import 'dashboard_tab.dart';
import 'files_tab.dart';
import 'ai_tab.dart';
import 'settings_tab.dart';

class HomeScreen extends StatefulWidget {
  const HomeScreen({super.key});

  @override
  State<HomeScreen> createState() => _HomeScreenState();
}

class _HomeScreenState extends State<HomeScreen> {
  int _currentIndex = 0;

  // Tab order: EXPLORER / SYNC / AI / MORE — business logic preserved
  final List<Widget> _tabs = [
    const FilesTab(),
    const DashboardTab(),
    const AiTab(),
    const SettingsTab(),
  ];

  static const List<_NavItem> _navItems = [
    _NavItem(icon: Icons.folder_outlined,   activeIcon: Icons.folder,             label: 'EXPLORER'),
    _NavItem(icon: Icons.sync_outlined,      activeIcon: Icons.sync,               label: 'SYNC'),
    _NavItem(icon: Icons.terminal_outlined,  activeIcon: Icons.terminal,           label: 'AI'),
    _NavItem(icon: Icons.grid_view_outlined, activeIcon: Icons.grid_view_rounded,  label: 'MORE'),
  ];

  @override
  Widget build(BuildContext context) {
    final syncProvider = context.watch<SyncProvider>();

    return Scaffold(
      backgroundColor: AppColors.bg,
      appBar: AppBar(
        backgroundColor: AppColors.bg,
        elevation: 0,
        titleSpacing: 16,
        title: const Row(
          children: [
            Icon(Icons.cloud_outlined, color: AppColors.textPrimary, size: 20),
            SizedBox(width: 8),
            Text('UNITEOS', style: TextStyle(
              fontWeight: FontWeight.w700,
              fontSize: 14,
              letterSpacing: 2,
              color: AppColors.textPrimary,
            )),
          ],
        ),
        actions: [
          Container(
            margin: const EdgeInsets.only(right: 12),
            padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
            decoration: BoxDecoration(
              border: Border.all(
                color: syncProvider.isConnected ? AppColors.blue : AppColors.border,
                width: 1.5,
              ),
            ),
            child: Row(
              mainAxisSize: MainAxisSize.min,
              children: [
                Icon(Icons.lock_outline, size: 12,
                  color: syncProvider.isConnected ? AppColors.blue : AppColors.textSecondary),
                const SizedBox(width: 6),
                Text(
                  syncProvider.isConnected ? 'SYNCED' : 'OFFLINE',
                  style: TextStyle(
                    fontSize: 10,
                    fontWeight: FontWeight.w700,
                    letterSpacing: 1.5,
                    color: syncProvider.isConnected ? AppColors.blue : AppColors.textSecondary,
                  ),
                ),
              ],
            ),
          ),
          IconButton(
            icon: const Icon(Icons.settings_outlined, size: 20),
            color: AppColors.textSecondary,
            onPressed: () => setState(() => _currentIndex = 3),
          ),
          const SizedBox(width: 4),
        ],
        bottom: PreferredSize(
          preferredSize: const Size.fromHeight(1),
          child: Container(height: 1, color: AppColors.border),
        ),
      ),
      body: IndexedStack(
        index: _currentIndex,
        children: _tabs,
      ),
      bottomNavigationBar: Container(
        decoration: const BoxDecoration(
          color: AppColors.bg,
          border: Border(top: BorderSide(color: AppColors.border, width: 1)),
        ),
        child: SafeArea(
          child: SizedBox(
            height: 64,
            child: Row(
              children: List.generate(_navItems.length, (i) {
                final item = _navItems[i];
                final isActive = _currentIndex == i;
                return Expanded(
                  child: GestureDetector(
                    onTap: () => setState(() => _currentIndex = i),
                    behavior: HitTestBehavior.opaque,
                    child: Column(
                      mainAxisAlignment: MainAxisAlignment.center,
                      children: [
                        Icon(
                          isActive ? item.activeIcon : item.icon,
                          size: 22,
                          color: isActive ? AppColors.yellow : AppColors.textMuted,
                        ),
                        const SizedBox(height: 4),
                        Text(
                          item.label,
                          style: TextStyle(
                            fontSize: 9,
                            fontWeight: FontWeight.w700,
                            letterSpacing: 1,
                            color: isActive ? AppColors.yellow : AppColors.textMuted,
                          ),
                        ),
                        const SizedBox(height: 4),
                        AnimatedContainer(
                          duration: const Duration(milliseconds: 200),
                          height: 2,
                          width: isActive ? 24 : 0,
                          color: AppColors.yellow,
                        ),
                      ],
                    ),
                  ),
                );
              }),
            ),
          ),
        ),
      ),
    );
  }
}

class _NavItem {
  final IconData icon;
  final IconData activeIcon;
  final String label;
  const _NavItem({required this.icon, required this.activeIcon, required this.label});
}

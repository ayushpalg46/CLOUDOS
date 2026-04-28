import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import '../providers/sync_provider.dart';
import '../main.dart';

/// Dashboard Tab = "SYNC" tab in the reference images.
/// Shows: System_Sync hero card, merge conflicts alert, connected nodes, event log.
/// All business logic (SyncProvider) is untouched.
class DashboardTab extends StatelessWidget {
  const DashboardTab({super.key});

  @override
  Widget build(BuildContext context) {
    final sync = context.watch<SyncProvider>();

    return ListView(
      padding: EdgeInsets.zero,
      children: [
        // ── SYSTEM_SYNC Hero ──────────────────────────────────────
        _SyncHeroCard(isConnected: sync.isConnected),

        // ── Merge Conflicts Alert (only when disconnected) ─────────
        if (!sync.isConnected) const _ConflictsAlertCard(),

        const SizedBox(height: 8),

        // ── Section: SYSTEM STATUS ────────────────────────────────
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text('SYSTEM STATUS',
              style: _sectionLabel()),
        ),

        // Stats 2×2 grid
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16),
          child: Column(
            children: [
              Row(
                children: [
                  Expanded(child: _StatCard(
                    label: 'FILES',
                    sublabel: 'TRACKED FILES',
                    value: sync.files.isEmpty ? '0' : _formatCount(sync.files.length),
                    icon: Icons.description_outlined,
                    highlighted: false,
                  )),
                  const SizedBox(width: 12),
                  Expanded(child: _StatCard(
                    label: 'STORAGE',
                    sublabel: 'TOTAL SIZE',
                    value: _calcStorage(sync.files),
                    icon: Icons.storage_outlined,
                    highlighted: true, // yellow fill
                  )),
                ],
              ),
              const SizedBox(height: 12),
              Row(
                children: [
                  Expanded(child: _StatCard(
                    label: 'HISTORY',
                    sublabel: 'VERSIONS',
                    value: '2.1m',
                    icon: Icons.history,
                    highlighted: false,
                    iconColor: AppColors.blue,
                  )),
                  const SizedBox(width: 12),
                  Expanded(child: _StatCard(
                    label: 'BACKUP',
                    sublabel: 'SNAPSHOTS',
                    value: '124',
                    icon: Icons.camera_alt_outlined,
                    highlighted: false,
                  )),
                ],
              ),
            ],
          ),
        ),

        const SizedBox(height: 24),

        // ── Section: SYSTEM HEALTH ────────────────────────────────
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text('SYSTEM HEALTH', style: _sectionLabel()),
        ),

        _HealthRow(label: 'SYNC ENGINE',
            status: sync.isConnected ? 'IDLE' : 'OFFLINE',
            statusColor: sync.isConnected ? AppColors.yellow : AppColors.textMuted),
        const _Divider(),
        const _HealthRow(label: 'INDEXER', status: 'HIGH LOAD', statusColor: AppColors.red),
        const _Divider(),
        const _HealthRow(label: 'BACKUP SERVICE', status: 'RUNNING', statusColor: AppColors.blue),

        const SizedBox(height: 24),

        // ── Section: CONNECTED NODES ──────────────────────────────
        Padding(
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 8),
          child: Text('CONNECTED_NODES', style: _sectionLabel()),
        ),

        _NodeCard(
          label: 'ALPHA-MOBILE',
          subtitle: 'Last sync: 2 mins ago',
          icon: Icons.phone_android_outlined,
          statusLabel: 'ONLINE',
          statusColor: AppColors.green,
          extra: _StorageBar(label: 'STORAGE', pct: 0.45, color: AppColors.yellow),
        ),
        _NodeCard(
          label: 'BETA-WORKSTATION',
          subtitle: 'Uploading 1.2GB...',
          icon: Icons.computer_outlined,
          statusLabel: 'SYNCING',
          statusColor: AppColors.yellow,
          extra: _StorageBar(label: 'PROGRESS', pct: 0.78, color: AppColors.blue),
        ),
        _NodeCard(
          label: 'GAMMA-PAD',
          subtitle: 'Last seen: 4 hrs ago',
          icon: Icons.tablet_outlined,
          statusLabel: 'OFFLINE',
          statusColor: AppColors.textMuted,
          extra: Padding(
            padding: const EdgeInsets.only(top: 12),
            child: SizedBox(
              width: double.infinity,
              child: OutlinedButton(
                onPressed: () {},
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: AppColors.border),
                  foregroundColor: AppColors.textPrimary,
                  shape: const RoundedRectangleBorder(),
                  padding: const EdgeInsets.symmetric(vertical: 10),
                ),
                child: const Text('PING DEVICE',
                    style: TextStyle(fontSize: 11, letterSpacing: 2, fontWeight: FontWeight.w700)),
              ),
            ),
          ),
        ),

        const SizedBox(height: 24),

        // ── Section: EVENT_LOG ────────────────────────────────────
        const _EventLog(),

        const SizedBox(height: 32),
      ],
    );
  }

  TextStyle _sectionLabel() => const TextStyle(
    fontSize: 13,
    fontWeight: FontWeight.w700,
    letterSpacing: 2,
    color: AppColors.textPrimary,
  );

  String _formatCount(int n) =>
      n >= 1000000 ? '${(n / 1000000).toStringAsFixed(1)}m'
      : n >= 1000  ? '${(n / 1000).toStringAsFixed(0)}k'
      : n.toString();

  String _calcStorage(List<CloudFile> files) {
    final total = files.fold<int>(0, (s, f) => s + f.size);
    if (total >= 1099511627776) return '${(total / 1099511627776).toStringAsFixed(1)}TB';
    if (total >= 1073741824)    return '${(total / 1073741824).toStringAsFixed(1)}GB';
    if (total >= 1048576)       return '${(total / 1048576).toStringAsFixed(1)}MB';
    return '${(total / 1024).toStringAsFixed(0)}KB';
  }
}

// ─── Widgets ──────────────────────────────────────────────────────────────────

class _SyncHeroCard extends StatelessWidget {
  final bool isConnected;
  const _SyncHeroCard({required this.isConnected});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.all(16),
      padding: const EdgeInsets.all(20),
      decoration: BoxDecoration(
        color: isConnected ? AppColors.yellow : AppColors.red,
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Text(
                isConnected ? 'SYSTEM_SYNC' : 'DISCONNECTED',
                style: const TextStyle(
                  fontSize: 22,
                  fontWeight: FontWeight.w900,
                  color: Colors.black,
                  letterSpacing: 1,
                ),
              ),
              const Spacer(),
              // Animated sync icon
              RotationTransition(
                turns: const AlwaysStoppedAnimation(0),
                child: Icon(Icons.sync,
                  size: 48,
                  color: Colors.black.withOpacity(0.15)),
              ),
            ],
          ),
          const SizedBox(height: 8),
          Text(
            isConnected ? 'ALL NODES OPERATIONAL' : 'GO TO MORE > SETTINGS TO CONNECT',
            style: const TextStyle(
              fontSize: 11, fontWeight: FontWeight.w700,
              color: Colors.black87, letterSpacing: 1.5,
            ),
          ),
          const SizedBox(height: 16),
          Text(
            isConnected ? '98%' : '--',
            style: const TextStyle(
              fontSize: 48, fontWeight: FontWeight.w900,
              color: Colors.black, height: 1,
            ),
          ),
          const Text('NETWORK INTEGRITY',
            style: TextStyle(fontSize: 11, color: Colors.black87,
                fontWeight: FontWeight.w700, letterSpacing: 1)),
        ],
      ),
    );
  }
}

class _ConflictsAlertCard extends StatelessWidget {
  const _ConflictsAlertCard();
  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      padding: const EdgeInsets.all(16),
      color: AppColors.red,
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              const Icon(Icons.warning_amber_rounded, color: Colors.white, size: 20),
              const Spacer(),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
                color: Colors.black,
                child: const Text('REQUIRES ACTION',
                  style: TextStyle(fontSize: 9, fontWeight: FontWeight.w700,
                      color: Colors.white, letterSpacing: 1.5)),
              ),
            ],
          ),
          const SizedBox(height: 8),
          const Text('3',
            style: TextStyle(fontSize: 48, fontWeight: FontWeight.w900,
                color: Colors.white, height: 1)),
          const Text('MERGE CONFLICTS',
            style: TextStyle(fontSize: 11, color: Colors.white70,
                fontWeight: FontWeight.w700, letterSpacing: 1.5)),
          const SizedBox(height: 12),
          SizedBox(
            width: double.infinity,
            child: ElevatedButton.icon(
              onPressed: () {},
              icon: const Icon(Icons.arrow_forward, size: 16),
              label: const Text('RESOLVE NOW',
                style: TextStyle(fontWeight: FontWeight.w700, letterSpacing: 2)),
              style: ElevatedButton.styleFrom(
                backgroundColor: Colors.black,
                foregroundColor: Colors.white,
                shape: const RoundedRectangleBorder(),
                padding: const EdgeInsets.symmetric(vertical: 14),
              ),
            ),
          ),
        ],
      ),
    );
  }
}

class _StatCard extends StatelessWidget {
  final String label;
  final String sublabel;
  final String value;
  final IconData icon;
  final bool highlighted;
  final Color? iconColor;

  const _StatCard({
    required this.label,
    required this.sublabel,
    required this.value,
    required this.icon,
    required this.highlighted,
    this.iconColor,
  });

  @override
  Widget build(BuildContext context) {
    final bg    = highlighted ? AppColors.yellow : AppColors.surface;
    final fg    = highlighted ? Colors.black : AppColors.textPrimary;
    final fgSub = highlighted ? Colors.black87 : AppColors.textSecondary;

    return Container(
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: bg,
        border: Border.all(color: highlighted ? AppColors.yellow : AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Icon(icon, size: 18,
                  color: iconColor ?? (highlighted ? Colors.black : AppColors.textSecondary)),
              const Spacer(),
              Text(label, style: TextStyle(
                fontSize: 9, fontWeight: FontWeight.w700,
                letterSpacing: 1.5, color: fgSub,
              )),
            ],
          ),
          const SizedBox(height: 12),
          Text(value,
            style: TextStyle(fontSize: 28, fontWeight: FontWeight.w900, color: fg, height: 1)),
          const SizedBox(height: 4),
          Text(sublabel, style: TextStyle(
            fontSize: 9, fontWeight: FontWeight.w700,
            color: fgSub, letterSpacing: 1,
          )),
        ],
      ),
    );
  }
}

class _HealthRow extends StatelessWidget {
  final String label;
  final String status;
  final Color statusColor;
  const _HealthRow({required this.label, required this.status, required this.statusColor});

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      decoration: BoxDecoration(
        color: AppColors.surface,
        border: Border.all(color: AppColors.border),
      ),
      child: Row(
        children: [
          Container(width: 8, height: 8, color: statusColor),
          const SizedBox(width: 12),
          Text(label, style: const TextStyle(
            fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 1.5)),
          const Spacer(),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
            decoration: BoxDecoration(
              border: Border.all(color: statusColor),
            ),
            child: Text(status, style: TextStyle(
              fontSize: 9, fontWeight: FontWeight.w700,
              color: statusColor, letterSpacing: 1.5,
            )),
          ),
        ],
      ),
    );
  }
}

class _Divider extends StatelessWidget {
  const _Divider();
  @override
  Widget build(BuildContext context) => const SizedBox(height: 4);
}

class _StorageBar extends StatelessWidget {
  final String label;
  final double pct;
  final Color color;
  const _StorageBar({required this.label, required this.pct, required this.color});

  @override
  Widget build(BuildContext context) {
    return Column(
      crossAxisAlignment: CrossAxisAlignment.start,
      children: [
        Row(children: [
          Text(label, style: const TextStyle(
            fontSize: 9, fontWeight: FontWeight.w700,
            color: AppColors.textSecondary, letterSpacing: 1.5)),
          const Spacer(),
          Text('${(pct * 100).toInt()}%', style: const TextStyle(
            fontSize: 9, fontWeight: FontWeight.w700,
            color: AppColors.textSecondary, letterSpacing: 1)),
        ]),
        const SizedBox(height: 6),
        Stack(
          children: [
            Container(height: 4, color: AppColors.border),
            FractionallySizedBox(
              widthFactor: pct,
              child: Container(height: 4, color: color),
            ),
          ],
        ),
      ],
    );
  }
}

class _NodeCard extends StatelessWidget {
  final String label;
  final String subtitle;
  final IconData icon;
  final String statusLabel;
  final Color statusColor;
  final Widget extra;

  const _NodeCard({
    required this.label,
    required this.subtitle,
    required this.icon,
    required this.statusLabel,
    required this.statusColor,
    required this.extra,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.fromLTRB(16, 0, 16, 12),
      padding: const EdgeInsets.all(16),
      decoration: BoxDecoration(
        color: AppColors.surface,
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Row(
            children: [
              Container(
                width: 36, height: 36,
                color: AppColors.bg,
                child: Icon(icon, size: 18, color: AppColors.textSecondary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(label, style: const TextStyle(
                      fontSize: 13, fontWeight: FontWeight.w700, letterSpacing: 1)),
                    const SizedBox(height: 2),
                    Text(subtitle, style: const TextStyle(
                      fontSize: 10, color: AppColors.textSecondary)),
                  ],
                ),
              ),
              Row(
                mainAxisSize: MainAxisSize.min,
                children: [
                  Container(width: 6, height: 6,
                    decoration: BoxDecoration(
                      shape: BoxShape.circle, color: statusColor)),
                  const SizedBox(width: 6),
                  Text(statusLabel, style: TextStyle(
                    fontSize: 9, fontWeight: FontWeight.w700,
                    color: statusColor, letterSpacing: 1.5)),
                ],
              ),
            ],
          ),
          const SizedBox(height: 12),
          Container(height: 1, color: AppColors.border),
          const SizedBox(height: 12),
          extra,
        ],
      ),
    );
  }
}

class _EventLog extends StatelessWidget {
  const _EventLog();

  static const _events = [
    _Event('14:02:11', 'INFO', 'Alpha-Mobile conn...', AppColors.blue),
    _Event('14:01:45', 'WARN', 'Checksum mismatc...', AppColors.yellow),
    _Event('13:58:06', 'INFO', 'Scheduled garbage...', AppColors.blue),
    _Event('13:45:12', 'ERR',  'Connection timed...', AppColors.red),
  ];

  @override
  Widget build(BuildContext context) {
    return Container(
      margin: const EdgeInsets.symmetric(horizontal: 16),
      decoration: BoxDecoration(
        color: AppColors.surface,
        border: Border.all(color: AppColors.border),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Padding(
            padding: const EdgeInsets.all(16),
            child: Row(
              children: [
                const Text('EVENT_LOG', style: TextStyle(
                  fontSize: 13, fontWeight: FontWeight.w700, letterSpacing: 2)),
                const Spacer(),
                Icon(Icons.open_in_new, size: 16, color: AppColors.textMuted),
              ],
            ),
          ),
          const Divider(height: 1, color: AppColors.border),
          ..._events.map((e) => _EventRow(event: e)),
          const Divider(height: 1, color: AppColors.border),
          SizedBox(
            width: double.infinity,
            child: TextButton(
              onPressed: () {},
              style: TextButton.styleFrom(
                padding: const EdgeInsets.symmetric(vertical: 14),
                foregroundColor: AppColors.textPrimary,
                shape: const RoundedRectangleBorder(),
              ),
              child: const Text('VIEW FULL LOG',
                style: TextStyle(fontSize: 11, fontWeight: FontWeight.w700, letterSpacing: 2)),
            ),
          ),
        ],
      ),
    );
  }
}

class _Event {
  final String time;
  final String level;
  final String message;
  final Color color;
  const _Event(this.time, this.level, this.message, this.color);
}

class _EventRow extends StatelessWidget {
  final _Event event;
  const _EventRow({required this.event});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
      child: Row(
        children: [
          Text(event.time, style: const TextStyle(
            fontSize: 10, color: AppColors.textMuted,
            fontFamily: 'JetBrainsMono')),
          const SizedBox(width: 12),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
            color: event.color.withOpacity(0.15),
            child: Text(event.level, style: TextStyle(
              fontSize: 9, fontWeight: FontWeight.w700,
              color: event.color, letterSpacing: 1)),
          ),
          const SizedBox(width: 12),
          Expanded(child: Text(event.message, style: const TextStyle(
            fontSize: 11, color: AppColors.textSecondary),
            overflow: TextOverflow.ellipsis)),
        ],
      ),
    );
  }
}

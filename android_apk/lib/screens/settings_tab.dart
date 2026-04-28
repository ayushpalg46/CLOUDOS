import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../providers/sync_provider.dart';
import '../main.dart';

/// Settings Tab = "MORE" tab.
/// Merges the Security Dashboard + Snapshots + Connection settings
/// shown across the reference design images.
/// All SyncProvider connection logic is preserved unchanged.
class SettingsTab extends StatefulWidget {
  const SettingsTab({super.key});

  @override
  State<SettingsTab> createState() => _SettingsTabState();
}

class _SettingsTabState extends State<SettingsTab> {
  final TextEditingController _ipController = TextEditingController();

  @override
  Widget build(BuildContext context) {
    final syncProvider = context.watch<SyncProvider>();

    return ListView(
      padding: EdgeInsets.zero,
      children: [
        // ── Section: SECURITY DASHBOARD ───────────────────────────
        const _SectionHeader('SECURITY DASHBOARD'),

        // Encryption Status card
        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 12),
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
                    const Text('ENCRYPTION STATUS', style: TextStyle(
                      fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 2)),
                    const Spacer(),
                    Icon(Icons.lock_open_outlined,
                      size: 16,
                      color: syncProvider.isConnected
                          ? AppColors.yellow : AppColors.textMuted),
                  ],
                ),
              ),
              const Divider(height: 1, color: AppColors.border),
              Container(
                width: double.infinity,
                margin: const EdgeInsets.all(16),
                padding: const EdgeInsets.all(20),
                color: AppColors.yellow,
                child: Column(
                  children: const [
                    Text('PROTOCOL ACTIVE', style: TextStyle(
                      fontSize: 9, fontWeight: FontWeight.w700,
                      color: Colors.black87, letterSpacing: 2)),
                    SizedBox(height: 8),
                    Text('AES-\n256', textAlign: TextAlign.center,
                      style: TextStyle(fontSize: 40, fontWeight: FontWeight.w900,
                          color: Colors.black, height: 1)),
                    SizedBox(height: 8),
                    Text('Military Grade End-to-End', style: TextStyle(
                      fontSize: 10, color: Colors.black87, fontWeight: FontWeight.w700)),
                  ],
                ),
              ),
              Padding(
                padding: const EdgeInsets.fromLTRB(16, 0, 16, 16),
                child: SizedBox(
                  width: double.infinity,
                  child: OutlinedButton(
                    onPressed: () {},
                    style: OutlinedButton.styleFrom(
                      side: const BorderSide(color: AppColors.border),
                      foregroundColor: AppColors.textPrimary,
                      shape: const RoundedRectangleBorder(),
                      padding: const EdgeInsets.symmetric(vertical: 14),
                    ),
                    child: const Text('VERIFY KEY', style: TextStyle(
                      fontSize: 11, fontWeight: FontWeight.w700, letterSpacing: 2)),
                  ),
                ),
              ),
            ],
          ),
        ),

        // Active Identity card
        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 24),
          decoration: BoxDecoration(
            color: AppColors.surface,
            border: Border.all(color: AppColors.border),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Padding(
                padding: EdgeInsets.all(16),
                child: Text('ACTIVE IDENTITY', style: TextStyle(
                  fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 2)),
              ),
              const Divider(height: 1, color: AppColors.border),
              _IdentityRow(
                icon: Icons.phone_android_outlined,
                iconColor: AppColors.blue,
                label: 'DEVICE NAME',
                value: 'Android Device',
              ),
              const Divider(height: 1, color: AppColors.border),
              _IdentityRow(
                icon: Icons.fingerprint_outlined,
                iconColor: AppColors.textMuted,
                label: 'SESSION ID',
                value: syncProvider.isConnected
                    ? syncProvider.targetIp.hashCode.toRadixString(16).substring(0, 8)
                    : 'x9A·4bF...7vQ',
              ),
              const Divider(height: 1, color: AppColors.border),
              const _IdentityRow(
                icon: Icons.location_on_outlined,
                iconColor: AppColors.textMuted,
                label: 'LOCATION',
                value: 'Local Network',
              ),
            ],
          ),
        ),

        // ── Section: INTEGRITY CHECKS ─────────────────────────────
        const _SectionHeader('INTEGRITY CHECKS'),

        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 24),
          decoration: BoxDecoration(
            color: AppColors.surface,
            border: Border.all(color: AppColors.border),
          ),
          child: Column(
            children: [
              _CheckRow(label: 'Bootloader Locked',
                  status: 'PASS', statusColor: AppColors.green,
                  icon: Icons.check_circle_outline),
              const Divider(height: 1, color: AppColors.border),
              _CheckRow(label: 'Filesystem Intact',
                  status: 'PASS', statusColor: AppColors.green,
                  icon: Icons.check_circle_outline),
              const Divider(height: 1, color: AppColors.border),
              _CheckRow(label: 'Outdated Definitions',
                  status: 'WARN', statusColor: AppColors.red,
                  icon: Icons.warning_amber_outlined),
            ],
          ),
        ),

        // ── Section: SECURE SHARE ─────────────────────────────────
        const _SectionHeader('SECURE SHARE'),

        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 24),
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: AppColors.yellow,
            border: Border.all(color: AppColors.yellow),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Generate time-limited, encrypted links for sensitive data transfer.',
                style: TextStyle(fontSize: 11, color: Colors.black87, height: 1.5)),
              const SizedBox(height: 12),
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 10),
                color: Colors.black.withOpacity(0.1),
                child: Row(
                  children: [
                    const Icon(Icons.link, size: 14, color: Colors.black87),
                    const SizedBox(width: 8),
                    Text(
                      syncProvider.isConnected
                          ? '${syncProvider.targetIp}/s/8f2a...'
                          : 'https://uniteos.link/s/8f2a...',
                      style: const TextStyle(fontSize: 11, color: Colors.black87,
                          fontFamily: 'JetBrainsMono'),
                    ),
                  ],
                ),
              ),
              const SizedBox(height: 10),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton.icon(
                  onPressed: () {},
                  icon: const Icon(Icons.share_outlined, size: 16),
                  label: const Text('GENERATE NEW LINK',
                    style: TextStyle(fontSize: 11, fontWeight: FontWeight.w800, letterSpacing: 1.5)),
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
        ),

        // ── Section: SNAPSHOTS ────────────────────────────────────
        const _SectionHeader('SNAPSHOTS'),
        const Padding(
          padding: EdgeInsets.fromLTRB(16, 0, 16, 12),
          child: Text('Point-in-time system backups.',
            style: TextStyle(fontSize: 11, color: AppColors.textMuted)),
        ),

        // New Snapshot button
        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 16),
          child: ElevatedButton.icon(
            onPressed: () {},
            icon: const Icon(Icons.add_circle_outline, size: 18),
            label: const Text('NEW SNAPSHOT',
              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w800, letterSpacing: 2)),
            style: ElevatedButton.styleFrom(
              backgroundColor: AppColors.yellow,
              foregroundColor: Colors.black,
              shape: const RoundedRectangleBorder(),
              padding: const EdgeInsets.symmetric(vertical: 16),
              minimumSize: const Size(double.infinity, 0),
            ),
          ),
        ),

        const _SnapshotCard(
          title: 'PRE-DEPLOYMENT\nBACKUP',
          date: 'Oct 24, 2023 – 14:30 UTC',
          type: 'SYSTEM',
          size: '12.4 GB',
          icon: Icons.history,
        ),
        const _SnapshotCard(
          title: 'WEEKLY AUTO-SYNC',
          date: 'Oct 20, 2023 – 00:00 UTC',
          type: 'AUTO',
          size: '11.8 GB',
          icon: Icons.update_outlined,
        ),
        const _SnapshotCard(
          title: 'INITIAL SETUP',
          date: 'Sep 01, 2023 – 09:15 UTC',
          type: 'MANUAL',
          size: '8.2 GB',
          icon: Icons.add_circle_outline,
        ),

        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 24),
          child: OutlinedButton(
            onPressed: () {},
            style: OutlinedButton.styleFrom(
              side: const BorderSide(color: AppColors.border, width: 1.5),
              foregroundColor: AppColors.textPrimary,
              shape: const RoundedRectangleBorder(),
              padding: const EdgeInsets.symmetric(vertical: 16),
            ),
            child: const Text('LOAD OLDER',
              style: TextStyle(fontSize: 12, fontWeight: FontWeight.w700, letterSpacing: 2)),
          ),
        ),

        // ── Section: CONNECT TO PC ────────────────────────────────
        const _SectionHeader('CONNECT TO PC NODE'),

        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 16),
          padding: const EdgeInsets.all(16),
          decoration: BoxDecoration(
            color: AppColors.surface,
            border: Border.all(color: AppColors.border),
          ),
          child: Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Text(
                'Enter the IP and Port of your Windows uniteOS server.',
                style: TextStyle(fontSize: 11, color: AppColors.textSecondary, height: 1.5)),
              const SizedBox(height: 16),
              TextField(
                controller: _ipController,
                decoration: InputDecoration(
                  hintText: '192.168.1.5:7890',
                  hintStyle: const TextStyle(color: AppColors.textMuted, fontSize: 12),
                  prefixIcon: const Icon(Icons.computer_outlined,
                    size: 18, color: AppColors.textMuted),
                  contentPadding: const EdgeInsets.symmetric(vertical: 14, horizontal: 12),
                  enabledBorder: const OutlineInputBorder(
                    borderRadius: BorderRadius.zero,
                    borderSide: BorderSide(color: AppColors.border)),
                  focusedBorder: const OutlineInputBorder(
                    borderRadius: BorderRadius.zero,
                    borderSide: BorderSide(color: AppColors.yellow, width: 1.5)),
                  filled: true,
                  fillColor: AppColors.bg,
                ),
                style: const TextStyle(fontSize: 12, fontFamily: 'JetBrainsMono'),
              ),
              const SizedBox(height: 12),
              SizedBox(
                width: double.infinity,
                child: ElevatedButton.icon(
                  onPressed: () {
                    if (_ipController.text.isNotEmpty) {
                      syncProvider.connect(_ipController.text);
                      ScaffoldMessenger.of(context).showSnackBar(
                        const SnackBar(content: Text('Attempting connection...')));
                    }
                  },
                  icon: const Icon(Icons.link, size: 16),
                  label: const Text('FORCE CONNECT', style: TextStyle(
                    fontSize: 11, fontWeight: FontWeight.w700, letterSpacing: 1.5)),
                  style: ElevatedButton.styleFrom(
                    backgroundColor: AppColors.yellow,
                    foregroundColor: Colors.black,
                    shape: const RoundedRectangleBorder(),
                    padding: const EdgeInsets.symmetric(vertical: 14),
                  ),
                ),
              ),
              if (syncProvider.isConnected) ...[
                const SizedBox(height: 12),
                Container(
                  padding: const EdgeInsets.all(12),
                  color: AppColors.green.withOpacity(0.1),
                  child: Row(
                    children: [
                      const Icon(Icons.check_circle_outline,
                        color: AppColors.green, size: 16),
                      const SizedBox(width: 10),
                      Expanded(
                        child: Text('SYNCED: ${syncProvider.targetIp}',
                          style: const TextStyle(
                            fontSize: 10, fontWeight: FontWeight.w700,
                            color: AppColors.green, letterSpacing: 1),
                          overflow: TextOverflow.ellipsis),
                      ),
                    ],
                  ),
                ),
              ],
            ],
          ),
        ),

        // ── Footer: SYSTEM SECURE ─────────────────────────────────
        Container(
          margin: const EdgeInsets.fromLTRB(16, 0, 16, 32),
          padding: const EdgeInsets.symmetric(vertical: 24),
          color: AppColors.yellow,
          alignment: Alignment.center,
          child: const Text('SYSTEM SECURE',
            style: TextStyle(fontSize: 20, fontWeight: FontWeight.w900,
                color: Colors.black, letterSpacing: 3)),
        ),
      ],
    );
  }

  @override
  void dispose() {
    _ipController.dispose();
    super.dispose();
  }
}

// ─── Widgets ──────────────────────────────────────────────────────────────────

class _SectionHeader extends StatelessWidget {
  final String title;
  const _SectionHeader(this.title);
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.fromLTRB(16, 24, 16, 12),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          Text(title, style: const TextStyle(
            fontSize: 20, fontWeight: FontWeight.w900,
            color: AppColors.textPrimary, letterSpacing: 1)),
          const SizedBox(height: 8),
          Container(height: 2, width: 40, color: AppColors.yellow),
        ],
      ),
    );
  }
}

class _IdentityRow extends StatelessWidget {
  final IconData icon;
  final Color iconColor;
  final String label;
  final String value;
  const _IdentityRow({
    required this.icon, required this.iconColor,
    required this.label, required this.value});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.all(16),
      child: Row(
        children: [
          Container(
            width: 36, height: 36,
            color: AppColors.bg,
            child: Icon(icon, size: 18, color: iconColor),
          ),
          const SizedBox(width: 12),
          Column(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Text(label, style: const TextStyle(
                fontSize: 8, fontWeight: FontWeight.w700,
                color: AppColors.textMuted, letterSpacing: 1.5)),
              const SizedBox(height: 2),
              Text(value, style: const TextStyle(
                fontSize: 13, fontWeight: FontWeight.w600)),
            ],
          ),
        ],
      ),
    );
  }
}

class _CheckRow extends StatelessWidget {
  final String label;
  final String status;
  final Color statusColor;
  final IconData icon;
  const _CheckRow({
    required this.label, required this.status,
    required this.statusColor, required this.icon});

  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 14),
      child: Row(
        children: [
          Icon(icon, size: 18, color: statusColor),
          const SizedBox(width: 12),
          Expanded(child: Text(label, style: const TextStyle(fontSize: 12))),
          Container(
            padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 4),
            color: statusColor == AppColors.red
                ? AppColors.red
                : Colors.transparent,
            decoration: statusColor != AppColors.red
                ? BoxDecoration(border: Border.all(color: AppColors.border))
                : null,
            child: Text(status, style: TextStyle(
              fontSize: 9, fontWeight: FontWeight.w700,
              color: statusColor == AppColors.red ? Colors.white : statusColor,
              letterSpacing: 1.5)),
          ),
        ],
      ),
    );
  }
}

class _SnapshotCard extends StatelessWidget {
  final String title;
  final String date;
  final String type;
  final String size;
  final IconData icon;
  const _SnapshotCard({
    required this.title, required this.date,
    required this.type, required this.size, required this.icon});

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
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              Container(
                width: 44, height: 44,
                color: AppColors.bg,
                child: Icon(icon, size: 22, color: AppColors.textSecondary),
              ),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(title, style: const TextStyle(
                      fontSize: 13, fontWeight: FontWeight.w800, letterSpacing: 0.5)),
                    const SizedBox(height: 4),
                    Row(
                      children: [
                        const Icon(Icons.calendar_today_outlined, size: 10,
                            color: AppColors.textMuted),
                        const SizedBox(width: 4),
                        Text(date, style: const TextStyle(
                          fontSize: 10, color: AppColors.textMuted)),
                      ],
                    ),
                    const SizedBox(height: 8),
                    Row(
                      children: [
                        _Tag(type),
                        const SizedBox(width: 6),
                        _Tag(size),
                      ],
                    ),
                  ],
                ),
              ),
            ],
          ),
          const SizedBox(height: 12),
          Row(
            children: [
              Expanded(
                child: OutlinedButton(
                  onPressed: () {},
                  style: OutlinedButton.styleFrom(
                    side: const BorderSide(color: AppColors.border),
                    foregroundColor: AppColors.textPrimary,
                    shape: const RoundedRectangleBorder(),
                    padding: const EdgeInsets.symmetric(vertical: 12),
                  ),
                  child: const Text('RESTORE', style: TextStyle(
                    fontSize: 11, fontWeight: FontWeight.w700, letterSpacing: 2)),
                ),
              ),
              const SizedBox(width: 8),
              Container(
                width: 44, height: 44,
                decoration: BoxDecoration(
                  border: Border.all(color: AppColors.border)),
                child: const Icon(Icons.delete_outline,
                  size: 18, color: AppColors.textSecondary),
              ),
            ],
          ),
        ],
      ),
    );
  }
}

class _Tag extends StatelessWidget {
  final String text;
  const _Tag(this.text);
  @override
  Widget build(BuildContext context) {
    return Container(
      padding: const EdgeInsets.symmetric(horizontal: 8, vertical: 3),
      decoration: BoxDecoration(
        border: Border.all(color: AppColors.border)),
      child: Text(text, style: const TextStyle(
        fontSize: 9, fontWeight: FontWeight.w700,
        color: AppColors.textSecondary, letterSpacing: 1)),
    );
  }
}

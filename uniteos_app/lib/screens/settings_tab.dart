import 'package:flutter/material.dart';
import 'package:provider/provider.dart';

import '../providers/sync_provider.dart';

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

    return Padding(
      padding: const EdgeInsets.all(24.0),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.stretch,
        children: [
          const Icon(Icons.settings_input_antenna, size: 64, color: Colors.blueAccent),
          const SizedBox(height: 24),
          Text(
            "Connect to PC Node",
            style: Theme.of(context).textTheme.headlineSmall?.copyWith(fontWeight: FontWeight.bold),
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 16),
          const Text(
            "Enter the IP and Port of your Windows UNITEos server to enable real-time synchronization.",
            textAlign: TextAlign.center,
          ),
          const SizedBox(height: 32),
          TextField(
            controller: _ipController,
            decoration: InputDecoration(
              labelText: "PC IP Address",
              hintText: "192.168.1.5:7890",
              prefixIcon: const Icon(Icons.computer),
              border: OutlineInputBorder(borderRadius: BorderRadius.circular(12)),
            ),
          ),
          const SizedBox(height: 24),
          ElevatedButton.icon(
            onPressed: () {
              if (_ipController.text.isNotEmpty) {
                syncProvider.connect(_ipController.text);
                ScaffoldMessenger.of(context).showSnackBar(const SnackBar(content: Text('Attempting connection...')));
              }
            },
            icon: const Icon(Icons.link),
            label: const Text("Force Connect"),
            style: ElevatedButton.styleFrom(
              padding: const EdgeInsets.symmetric(vertical: 16.0),
              shape: RoundedRectangleBorder(borderRadius: BorderRadius.circular(12)),
            ),
          ),
          const SizedBox(height: 48),
          if (syncProvider.isConnected)
            Card(
              color: Colors.greenAccent.withOpacity(0.1),
              child: const Padding(
                padding: EdgeInsets.all(16.0),
                child: Row(
                  children: [
                    Icon(Icons.check_circle, color: Colors.greenAccent),
                    SizedBox(width: 16),
                    Expanded(child: Text("Successfully synced with Master Node.")),
                  ],
                ),
              ),
            )
        ],
      ),
    );
  }
}

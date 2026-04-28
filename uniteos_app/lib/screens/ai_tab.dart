import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:http/http.dart' as http;

import '../providers/sync_provider.dart';

class AiTab extends StatefulWidget {
  const AiTab({super.key});

  @override
  State<AiTab> createState() => _AiTabState();
}

class _AiTabState extends State<AiTab> {
  final TextEditingController _controller = TextEditingController();
  final List<Map<String, String>> _messages = [];
  bool _isLoading = false;

  Future<void> _sendMessage(String targetIp) async {
    final text = _controller.text.trim();
    if (text.isEmpty || !context.read<SyncProvider>().isConnected) return;

    setState(() {
      _messages.add({"role": "user", "text": text});
      _isLoading = true;
    });
    _controller.clear();

    try {
      final response = await http.post(
        Uri.parse('$targetIp/api/chat'),
        headers: {"Content-Type": "application/json"},
        body: json.encode({"message": text}),
      );

      if (response.statusCode == 200) {
        final data = json.decode(response.body);
        setState(() {
          _messages.add({"role": "ai", "text": data['reply'] ?? 'Empty response.'});
        });
      } else {
        setState(() {
          _messages.add({"role": "ai", "text": "Error: Could not reach AI server."});
        });
      }
    } catch (e) {
      setState(() {
        _messages.add({"role": "ai", "text": "Connection failed."});
      });
    } finally {
      setState(() {
        _isLoading = false;
      });
    }
  }

  @override
  Widget build(BuildContext context) {
    final syncProvider = context.watch<SyncProvider>();

    return Column(
      children: [
        if (!syncProvider.isConnected)
          Container(
            color: Colors.redAccent.withOpacity(0.2),
            width: double.infinity,
            padding: const EdgeInsets.all(8.0),
            child: const Text(
              "Connect to PC to use AI features.",
              textAlign: TextAlign.center,
              style: TextStyle(color: Colors.redAccent),
            ),
          ),
        Expanded(
          child: ListView.builder(
            padding: const EdgeInsets.all(16.0),
            itemCount: _messages.length,
            itemBuilder: (context, index) {
              final msg = _messages[index];
              final isUser = msg['role'] == 'user';
              return Align(
                alignment: isUser ? Alignment.centerRight : Alignment.centerLeft,
                child: Container(
                  margin: const EdgeInsets.symmetric(vertical: 4.0),
                  padding: const EdgeInsets.symmetric(horizontal: 16.0, vertical: 12.0),
                  decoration: BoxDecoration(
                    color: isUser ? Theme.of(context).colorScheme.primary : Theme.of(context).cardColor,
                    borderRadius: BorderRadius.circular(16.0),
                  ),
                  child: Text(
                    msg['text']!,
                    style: TextStyle(color: isUser ? Colors.white : Colors.white70),
                  ),
                ),
              );
            },
          ),
        ),
        if (_isLoading) const Padding(padding: EdgeInsets.all(8.0), child: CircularProgressIndicator()),
        Padding(
          padding: const EdgeInsets.all(16.0),
          child: Row(
            children: [
              Expanded(
                child: TextField(
                  controller: _controller,
                  decoration: InputDecoration(
                    hintText: "Ask UNITEos AI...",
                    filled: true,
                    fillColor: Theme.of(context).cardColor,
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(24.0),
                      borderSide: BorderSide.none,
                    ),
                    contentPadding: const EdgeInsets.symmetric(horizontal: 20.0, vertical: 14.0),
                  ),
                  onSubmitted: (_) => _sendMessage(syncProvider.targetIp),
                ),
              ),
              const SizedBox(width: 8),
              CircleAvatar(
                radius: 24,
                backgroundColor: Theme.of(context).colorScheme.primary,
                child: IconButton(
                  icon: const Icon(Icons.send, color: Colors.white),
                  onPressed: () => _sendMessage(syncProvider.targetIp),
                ),
              )
            ],
          ),
        ),
      ],
    );
  }
}

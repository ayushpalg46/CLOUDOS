import 'dart:convert';
import 'package:flutter/material.dart';
import 'package:provider/provider.dart';
import 'package:http/http.dart' as http;

import '../providers/sync_provider.dart';
import '../main.dart';

/// AI Tab = "AI" tab in the reference images.
/// Matches the terminal-style "CORE_SYSTEM / AI ASSISTANT" look.
/// All network/API logic is unchanged.
class AiTab extends StatefulWidget {
  const AiTab({super.key});

  @override
  State<AiTab> createState() => _AiTabState();
}

class _AiTabState extends State<AiTab> {
  final TextEditingController _controller = TextEditingController();
  final ScrollController _scrollController = ScrollController();
  // unchanged state
  final List<Map<String, String>> _messages = [];
  bool _isLoading = false;

  // ── unchanged business logic ──────────────────────────────────────────────
  Future<void> _sendMessage(String targetIp) async {
    final text = _controller.text.trim();
    if (text.isEmpty || !context.read<SyncProvider>().isConnected) return;

    setState(() {
      _messages.add({"role": "user", "text": text});
      _isLoading = true;
    });
    _controller.clear();
    _scrollToBottom();

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
      setState(() { _isLoading = false; });
      _scrollToBottom();
    }
  }

  void _scrollToBottom() {
    WidgetsBinding.instance.addPostFrameCallback((_) {
      if (_scrollController.hasClients) {
        _scrollController.animateTo(
          _scrollController.position.maxScrollExtent,
          duration: const Duration(milliseconds: 300),
          curve: Curves.easeOut,
        );
      }
    });
  }
  // ─────────────────────────────────────────────────────────────────────────

  @override
  Widget build(BuildContext context) {
    final syncProvider = context.watch<SyncProvider>();

    return Column(
      children: [
        // ── Top status bar ────────────────────────────────────────
        Container(
          width: double.infinity,
          padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 10),
          decoration: const BoxDecoration(
            border: Border(bottom: BorderSide(color: AppColors.border)),
          ),
          child: Row(
            children: [
              // LLM status chip
              Container(
                padding: const EdgeInsets.symmetric(horizontal: 12, vertical: 6),
                decoration: BoxDecoration(
                  color: syncProvider.isConnected
                      ? AppColors.red.withOpacity(0.15)
                      : AppColors.surface,
                  border: Border.all(
                    color: syncProvider.isConnected ? AppColors.red : AppColors.border),
                ),
                child: Row(
                  mainAxisSize: MainAxisSize.min,
                  children: [
                    Container(
                      width: 6, height: 6,
                      decoration: BoxDecoration(
                        shape: BoxShape.circle,
                        color: syncProvider.isConnected ? AppColors.red : AppColors.textMuted,
                      ),
                    ),
                    const SizedBox(width: 6),
                    Text(
                      syncProvider.isConnected ? 'LOCAL LLM STATUS: ONLINE' : 'LLM: OFFLINE',
                      style: TextStyle(
                        fontSize: 9, fontWeight: FontWeight.w700,
                        letterSpacing: 1.5,
                        color: syncProvider.isConnected ? AppColors.red : AppColors.textMuted,
                      ),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),

        // ── Chat messages ─────────────────────────────────────────
        Expanded(
          child: _messages.isEmpty
              ? _SystemInitCard()
              : ListView.builder(
                  controller: _scrollController,
                  padding: const EdgeInsets.all(16),
                  itemCount: _messages.length + (_isLoading ? 1 : 0),
                  itemBuilder: (context, index) {
                    if (index == _messages.length) {
                      return const _AiThinkingBubble();
                    }
                    final msg = _messages[index];
                    return msg['role'] == 'user'
                        ? _UserBubble(text: msg['text']!)
                        : _AiBubble(text: msg['text']!);
                  },
                ),
        ),

        // ── Saved Queries (shown when no connection) ──────────────
        if (!syncProvider.isConnected) const _SavedQueriesBar(),

        // ── Input bar ─────────────────────────────────────────────
        _InputBar(
          controller: _controller,
          isConnected: syncProvider.isConnected,
          onSend: () => _sendMessage(syncProvider.targetIp),
        ),
      ],
    );
  }

  @override
  void dispose() {
    _controller.dispose();
    _scrollController.dispose();
    super.dispose();
  }
}

// ─── Widgets ──────────────────────────────────────────────────────────────────

class _SystemInitCard extends StatelessWidget {
  @override
  Widget build(BuildContext context) {
    return ListView(
      padding: const EdgeInsets.all(16),
      children: [
        // System Initialized message
        Container(
          padding: const EdgeInsets.all(20),
          decoration: BoxDecoration(
            color: AppColors.blue.withOpacity(0.08),
            border: Border.all(color: AppColors.blue.withOpacity(0.4), width: 1.5),
          ),
          child: Row(
            crossAxisAlignment: CrossAxisAlignment.start,
            children: [
              const Icon(Icons.radio_button_checked, size: 18, color: AppColors.blue),
              const SizedBox(width: 12),
              Expanded(
                child: Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: const [
                    Text('SYSTEM INITIALIZED',
                      style: TextStyle(fontSize: 13, fontWeight: FontWeight.w800,
                          letterSpacing: 2, color: AppColors.textPrimary)),
                    SizedBox(height: 8),
                    Text(
                      'uniteOS AI Assistant active. Local LLM model loaded into VRAM. '
                      'Ready for terminal commands, script generation, and system analysis. '
                      'Awaiting input.',
                      style: TextStyle(fontSize: 12, color: AppColors.textSecondary, height: 1.5),
                    ),
                  ],
                ),
              ),
            ],
          ),
        ),
        const SizedBox(height: 24),
        // Saved queries header
        const Text('SAVED QUERIES',
          style: TextStyle(fontSize: 10, fontWeight: FontWeight.w700,
              color: AppColors.yellow, letterSpacing: 2)),
        const SizedBox(height: 8),
        const Divider(height: 1, color: AppColors.border),
        const SizedBox(height: 12),
        _QueryItem('Optimize DB indexing script'),
        _QueryItem('Generate API boilerplate'),
        _QueryItem('Explain Docker compose n...'),
      ],
    );
  }
}

class _QueryItem extends StatelessWidget {
  final String text;
  const _QueryItem(this.text);
  @override
  Widget build(BuildContext context) {
    return Padding(
      padding: const EdgeInsets.symmetric(vertical: 8),
      child: Text(text,
        style: const TextStyle(fontSize: 12, color: AppColors.textSecondary)),
    );
  }
}

class _UserBubble extends StatelessWidget {
  final String text;
  const _UserBubble({required this.text});

  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerRight,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12, left: 48),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              padding: const EdgeInsets.all(16),
              decoration: BoxDecoration(
                color: AppColors.surface,
                border: Border.all(color: AppColors.border),
              ),
              child: Text(text,
                style: const TextStyle(fontSize: 13, height: 1.5,
                    color: AppColors.textPrimary)),
            ),
            Positioned(
              top: -8, right: -8,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                color: AppColors.yellow,
                child: const Text('USER', style: TextStyle(
                  fontSize: 8, fontWeight: FontWeight.w800,
                  color: Colors.black, letterSpacing: 1)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AiBubble extends StatelessWidget {
  final String text;
  const _AiBubble({required this.text});

  @override
  Widget build(BuildContext context) {
    // Detect if message looks like a code block
    final hasCode = text.contains('\n') && (
        text.contains('find ') || text.contains('#!/') ||
        text.contains('import ') || text.contains('{'));

    return Align(
      alignment: Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.only(bottom: 16, right: 16),
        child: Stack(
          clipBehavior: Clip.none,
          children: [
            Container(
              padding: const EdgeInsets.all(16),
              width: double.infinity,
              decoration: BoxDecoration(
                color: AppColors.surface,
                border: Border.all(color: AppColors.yellow.withOpacity(0.6)),
              ),
              child: Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  // If multiline, try to split into prose + code
                  if (hasCode) ...[
                    Text(text.split('\n').first,
                      style: const TextStyle(fontSize: 13, height: 1.5)),
                    const SizedBox(height: 12),
                    // Code block
                    Container(
                      width: double.infinity,
                      padding: const EdgeInsets.all(12),
                      color: Colors.black,
                      child: Text(
                        text.split('\n').skip(1).join('\n').trim(),
                        style: const TextStyle(
                          fontSize: 11, fontFamily: 'JetBrainsMono',
                          color: AppColors.textPrimary, height: 1.6),
                      ),
                    ),
                  ] else
                    Text(text,
                      style: const TextStyle(fontSize: 13, height: 1.5)),
                ],
              ),
            ),
            Positioned(
              top: -8, left: 0,
              child: Container(
                padding: const EdgeInsets.symmetric(horizontal: 6, vertical: 2),
                color: AppColors.yellow,
                child: const Text('ASSISTANT', style: TextStyle(
                  fontSize: 8, fontWeight: FontWeight.w800,
                  color: Colors.black, letterSpacing: 1)),
              ),
            ),
          ],
        ),
      ),
    );
  }
}

class _AiThinkingBubble extends StatelessWidget {
  const _AiThinkingBubble();
  @override
  Widget build(BuildContext context) {
    return Align(
      alignment: Alignment.centerLeft,
      child: Container(
        margin: const EdgeInsets.only(bottom: 12),
        padding: const EdgeInsets.symmetric(horizontal: 16, vertical: 12),
        decoration: BoxDecoration(
          color: AppColors.surface,
          border: Border.all(color: AppColors.yellow.withOpacity(0.4)),
        ),
        child: const Row(
          mainAxisSize: MainAxisSize.min,
          children: [
            SizedBox(
              width: 14, height: 14,
              child: CircularProgressIndicator(
                strokeWidth: 2, color: AppColors.yellow)),
            SizedBox(width: 10),
            Text('PROCESSING...', style: TextStyle(
              fontSize: 10, fontWeight: FontWeight.w700,
              color: AppColors.yellow, letterSpacing: 2)),
          ],
        ),
      ),
    );
  }
}

class _SavedQueriesBar extends StatelessWidget {
  const _SavedQueriesBar();
  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: const BoxDecoration(
        border: Border(top: BorderSide(color: AppColors.border))),
      padding: const EdgeInsets.fromLTRB(16, 10, 16, 6),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          const Text('SAVED QUERIES', style: TextStyle(
            fontSize: 9, fontWeight: FontWeight.w700,
            color: AppColors.yellow, letterSpacing: 2)),
          const SizedBox(height: 6),
          SingleChildScrollView(
            scrollDirection: Axis.horizontal,
            child: Row(
              children: [
                'Optimize DB indexing script',
                'Generate API boilerplate',
                'Explain Docker compose',
              ].map((q) => Container(
                margin: const EdgeInsets.only(right: 8),
                padding: const EdgeInsets.symmetric(horizontal: 10, vertical: 6),
                decoration: BoxDecoration(
                  border: Border.all(color: AppColors.border)),
                child: Text(q, style: const TextStyle(
                  fontSize: 10, color: AppColors.textSecondary)),
              )).toList(),
            ),
          ),
        ],
      ),
    );
  }
}

class _InputBar extends StatelessWidget {
  final TextEditingController controller;
  final bool isConnected;
  final VoidCallback onSend;

  const _InputBar({
    required this.controller,
    required this.isConnected,
    required this.onSend,
  });

  @override
  Widget build(BuildContext context) {
    return Container(
      decoration: const BoxDecoration(
        color: AppColors.surface,
        border: Border(top: BorderSide(color: AppColors.border)),
      ),
      child: Column(
        crossAxisAlignment: CrossAxisAlignment.start,
        children: [
          // Text input
          TextField(
            controller: controller,
            enabled: isConnected,
            maxLines: 3,
            minLines: 1,
            decoration: InputDecoration(
              hintText: isConnected
                  ? 'INITIALIZE QUERY...'
                  : 'CONNECT TO PC TO USE AI',
              hintStyle: const TextStyle(
                fontSize: 12, color: AppColors.textMuted,
                letterSpacing: 1, fontWeight: FontWeight.w600),
              contentPadding: const EdgeInsets.fromLTRB(16, 14, 16, 10),
              border: InputBorder.none,
            ),
            style: const TextStyle(fontSize: 13),
            onSubmitted: (_) => onSend(),
          ),
          Container(height: 1, color: AppColors.border),
          // Bottom action row
          Padding(
            padding: const EdgeInsets.fromLTRB(12, 8, 12, 12),
            child: Row(
              children: [
                IconButton(
                  icon: const Icon(Icons.attach_file, size: 18),
                  color: AppColors.textMuted,
                  onPressed: () {},
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(),
                ),
                const SizedBox(width: 12),
                IconButton(
                  icon: const Icon(Icons.language, size: 18),
                  color: AppColors.textMuted,
                  onPressed: () {},
                  padding: EdgeInsets.zero,
                  constraints: const BoxConstraints(),
                ),
                const Spacer(),
                // EXECUTE button
                GestureDetector(
                  onTap: onSend,
                  child: Container(
                    padding: const EdgeInsets.symmetric(horizontal: 20, vertical: 12),
                    decoration: BoxDecoration(
                      color: isConnected ? AppColors.yellow : AppColors.surface,
                      border: Border.all(
                        color: isConnected ? AppColors.yellow : AppColors.border),
                    ),
                    child: Row(
                      mainAxisSize: MainAxisSize.min,
                      children: [
                        Text('EXECUTE',
                          style: TextStyle(
                            fontSize: 11, fontWeight: FontWeight.w800,
                            letterSpacing: 2,
                            color: isConnected ? Colors.black : AppColors.textMuted,
                          ),
                        ),
                        const SizedBox(width: 8),
                        Icon(Icons.play_arrow,
                          size: 14,
                          color: isConnected ? Colors.black : AppColors.textMuted),
                      ],
                    ),
                  ),
                ),
              ],
            ),
          ),
        ],
      ),
    );
  }
}

#!/usr/bin/env perl
# Loads the Perl asciiquarium with Term::Animation and Curses stubbed out,
# calls every add_* sub with a mock animation object, and prints the shapes
# and color masks it would have drawn as JSON. Run via extract.sh.
# Dumps all art (shapes + color masks) from the Perl asciiquarium as JSON.
our $RAND_MODE = 0;   # 0 => rand returns 0, 1 => rand returns just under max
BEGIN {
  *CORE::GLOBAL::rand = sub { my $m = @_ ? $_[0] : 1; $main::RAND_MODE ? $m - 1e-9 : 0 };
  $INC{'Term/Animation.pm'}=1; $INC{'Term/Animation/Entity.pm'}=1; $INC{'Curses.pm'}=1;
  package Term::Animation; sub import {} sub VERSION {2}
  package Term::Animation::Entity; sub import {}
  package Curses; sub import {}
}
package MockAnim;
sub new { bless { ents=>[] }, shift }
sub width { 200 } sub height { 60 }
sub new_entity { my ($s,%a)=@_; push @{$s->{ents}}, \%a; }
sub add_entity { my ($s,$e)=@_; push @{$s->{ents}}, $e; }
package main; use JSON::PP ();
my $src = do { local $/; open my $f, '<', $ARGV[0] or die; <$f> };
$src =~ s/^main\(\);$//m;
$src =~ s/^use (Curses|Term::Animation\S*).*$//mg;
eval $src; die $@ if $@;
no warnings 'redefine';
*main::rand_color = sub { $_[0] };
my %out; my $cur;
*main::add_fish_entity = sub { my ($a,@img)=@_; $out{$cur} = \@img; };
for my $s (qw(add_new_fish add_old_fish)) { $cur=$s; no strict 'refs'; &{"main::$s"}(undef, MockAnim->new); }
sub norm { my $x=shift; ref $x eq 'ARRAY' ? [map {norm($_)} @$x] : $x }
for my $s (qw(add_environment add_castle add_shark add_fishhook add_ship add_whale add_new_monster add_old_monster add_big_fish_1 add_big_fish_2 add_ducks add_dolphins add_swan add_submarine add_sword_fish add_splat)) {
  for my $mode (0,1) {
    $RAND_MODE=$mode; my $a=MockAnim->new; no strict 'refs';
    eval { ($s eq "add_environment"||$s eq "add_castle") ? &{"main::$s"}($a) : $s eq "add_splat" ? &{"main::$s"}($a,10,10,10) : &{"main::$s"}(undef,$a) }; warn "$s: $@" if $@;
    $out{"$s/$mode"} = [ map { my $e=$_; +{ map { $_ => norm($e->{$_}) } grep { $_ ne 'callback' && $_ ne 'death_cb' && $_ ne 'coll_handler' } keys %$e } } @{$a->{ents}} ];
  }
}
print JSON::PP->new->canonical->pretty->encode(\%out);

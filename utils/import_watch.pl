#!/usr/bin/perl

use DBI;
use strict;

my $watch = $ARGV[0] || die "usage: $0 <watch.tab> <source_id>";
my $source_id = $ARGV[1];

my $stock;

my $dbh = DBI->connect(
  "dbi:CSV:", undef, undef, {
    f_ext      => ".txt/r",
    f_dir      => "data",
    RaiseError => 1,
  }
) or die "Cannot connect: $DBI::errstr";

#
# "ticker_id","ticker_symbol","exchange_id","name","country_id"
# "1","AAMC","1","Altisource Asset Management Corp Com","NULL"
# "2","AAU","1","Almaden Minerals Ltd. Common Shares","NULL"
# "3","ACU","1","Acme United Corporation. Common Stock","NULL"
# "4","ACY","1","AeroCentury Corp. Common Stock","NULL"
#
my $sth = $dbh->prepare("select * from ticker where exchange_id IN (1,15,18)");
$sth->execute;
$sth->bind_columns (\my ($ticker_id, $ticker_symbol, $exchange_id, $name, $country_id));
while ($sth->fetch) {
  $stock->{$ticker_symbol} = $ticker_id;
}
$sth->finish;
$dbh->disconnect;

my $dbh = DBI->connect(
  "dbi:CSV:", undef, undef, {
    f_ext          => ".tab/r",
    f_dir          => "data",
    sep_char       => "\t",
    RaiseError     => 1,
  }
) or die "Cannot connect: $DBI::errstr";

#
# 02-07-2021  RBC   GSHD  Financials  $155.00
# 02-07-2021  RBC   VRNS  Information Technology  $200.00
# 02-07-2021  RBC   PCRX  Healthcare  $78.00
# 02-07-2021  RBC   SAIL  Information Technology  $68.00
#
$sth = $dbh->prepare("select * from $watch");
$sth->execute;
$sth->bind_columns (\my ($source_date, $source_company, $source_individual, $ticker_symbol, $industry, $price));
while ($sth->fetch) {
  if (exists $stock->{$ticker_symbol}) {
    $price =~ s/\$//;
    my ($mm, $dd, $yyyy) = $source_date =~ /^(\d\d)\-(\d\d)\-(\d\d\d\d)$/;
    printf "INSERT INTO watch SET ticker_id=%d, source_id=$source_id, source_date='%s', target_price=%f; # $ticker_symbol\n",
      $stock->{$ticker_symbol}, "$yyyy-$mm-$dd", $price;
  }
  else {
    print "# ERROR - did not find symbol '$ticker_symbol'\n";
  }
}
$sth->finish;
 
$dbh->disconnect;

